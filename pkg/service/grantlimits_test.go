// Copyright 2024 LiveKit, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/livekit/livekit-server/pkg/config"
	"github.com/livekit/livekit-server/pkg/rtc"
	"github.com/livekit/protocol/auth"
	"github.com/livekit/protocol/livekit"
	"github.com/livekit/protocol/logger"
)

// stubRoomAllocator is a RoomAllocator whose ValidateCreateRoom always succeeds. The embedded
// interface is nil, so any other method panics, which keeps the tests honest about what the
// join path actually calls.
type stubRoomAllocator struct{ RoomAllocator }

func (stubRoomAllocator) ValidateCreateRoom(context.Context, livekit.RoomName) error { return nil }

func grantRequest(grant *auth.ClaimGrants) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	return r.WithContext(WithGrants(r.Context(), grant, "apikey"))
}

// ConnectLimitCase is a join token that is over one limit. Every join path must reject it.
// It is exported so that TestStartSessionConnectLimits in package service_test runs the same cases.
type ConnectLimitCase struct {
	Name  string
	Limit config.LimitConfig
	Grant func() *auth.ClaimGrants // fresh grant per call, since a join path may change it
	Err   error
}

func ConnectLimitCases() []ConnectLimitCase {
	return []ConnectLimitCase{
		{
			Name:  "identity exceeds",
			Limit: config.LimitConfig{MaxParticipantIdentityLength: 5},
			Grant: grantWith(func(g *auth.ClaimGrants) { g.Identity = "toolongidentity" }),
			Err:   ErrParticipantIdentityExceedsLimits,
		},
		{
			Name:  "room name exceeds",
			Limit: config.LimitConfig{MaxRoomNameLength: 3},
			Grant: grantWith(func(g *auth.ClaimGrants) { g.Video.Room = "toolongroom" }),
			Err:   ErrRoomNameExceedsLimits,
		},
		{
			Name:  "name exceeds",
			Limit: config.LimitConfig{MaxParticipantNameLength: 5},
			Grant: grantWith(func(g *auth.ClaimGrants) { g.Name = "toolongname" }),
			Err:   ErrNameExceedsLimits,
		},
		{
			Name:  "metadata exceeds",
			Limit: config.LimitConfig{MaxMetadataSize: 5},
			Grant: grantWith(func(g *auth.ClaimGrants) { g.Metadata = "abcdefg" }),
			Err:   ErrMetadataExceedsLimits,
		},
		{
			Name:  "attributes exceeds",
			Limit: config.LimitConfig{MaxAttributesSize: 5},
			Grant: grantWith(func(g *auth.ClaimGrants) { g.Attributes = map[string]string{"k": "abcdefg"} }),
			Err:   ErrAttributeExceedsLimits,
		},
		{
			Name:  "lk attributes exceeds",
			Limit: config.LimitConfig{MaxAttributesSize: 100, MaxLKAttributesSize: 5},
			Grant: grantWith(func(g *auth.ClaimGrants) { g.Attributes = map[string]string{"lk.k": "abcdefg"} }),
			Err:   ErrAttributeExceedsLimits,
		},
	}
}

func baseGrant() *auth.ClaimGrants {
	return &auth.ClaimGrants{
		Identity: "id",
		Name:     "name",
		Video:    &auth.VideoGrant{RoomJoin: true, Room: "room"},
	}
}

func grantWith(mutate func(*auth.ClaimGrants)) func() *auth.ClaimGrants {
	return func() *auth.ClaimGrants {
		g := baseGrant()
		mutate(g)
		return g
	}
}

// TestConnectLimits checks that both HTTP join paths (WHIP ingest and the signal connection via
// ValidateConnectRequest) reject every ConnectLimitCase.
func TestConnectLimits(t *testing.T) {
	for _, tc := range ConnectLimitCases() {
		t.Run("WHIP/"+tc.Name, func(t *testing.T) {
			svc := &WHIPService{config: &config.Config{Limit: tc.Limit}, roomAllocator: stubRoomAllocator{}}
			_, code, err := svc.validateCreate(httptest.NewRecorder(), grantRequest(tc.Grant()))
			require.ErrorIs(t, err, tc.Err)
			require.Equal(t, http.StatusBadRequest, code)
		})

		t.Run("signal/"+tc.Name, func(t *testing.T) {
			// nil router skips the node-limit check, reaching the grant checks
			_, code, err := ValidateConnectRequest(logger.GetLogger(), grantRequest(tc.Grant()), tc.Limit, ValidateConnectRequestParams{}, nil, stubRoomAllocator{})
			require.ErrorIs(t, err, tc.Err)
			require.Equal(t, http.StatusBadRequest, code)
		})
	}

	t.Run("signal/identity exceeds after publish suffix", func(t *testing.T) {
		g := baseGrant()
		g.Video.SetCanPublish(true)
		// "id" fits a limit of 5, but the final identity "id#abcd" does not
		params := ValidateConnectRequestParams{publish: "abcd"}
		_, code, err := ValidateConnectRequest(logger.GetLogger(), grantRequest(g), config.LimitConfig{MaxParticipantIdentityLength: 5}, params, nil, stubRoomAllocator{})
		require.ErrorIs(t, err, ErrParticipantIdentityExceedsLimits)
		require.Equal(t, http.StatusBadRequest, code)
	})

	t.Run("within limits", func(t *testing.T) {
		limit := config.LimitConfig{MaxParticipantIdentityLength: 5, MaxRoomNameLength: 10, MaxParticipantNameLength: 5, MaxMetadataSize: 5, MaxAttributesSize: 5}
		_, code, err := ValidateConnectRequest(logger.GetLogger(), grantRequest(baseGrant()), limit, ValidateConnectRequestParams{}, nil, stubRoomAllocator{})
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, code)
	})

	t.Run("lk attributes do not count against attributes limit", func(t *testing.T) {
		g := baseGrant()
		g.Attributes = map[string]string{"k": "abcd", "lk.k": "abcdefg"}
		limit := config.LimitConfig{MaxAttributesSize: 5, MaxLKAttributesSize: 11}
		_, code, err := ValidateConnectRequest(logger.GetLogger(), grantRequest(g), limit, ValidateConnectRequestParams{}, nil, stubRoomAllocator{})
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, code)
	})
}

// The room-limit exemption is only honored from the signed token. A client must not be able to
// grant it to itself through connect attributes (or overwrite the token's value).
func TestIgnoreRoomLimitAttributeReservedForToken(t *testing.T) {
	t.Run("client value cannot override the token", func(t *testing.T) {
		g := baseGrant()
		g.Video.SetCanUpdateOwnMetadata(true)
		g.Attributes = map[string]string{rtc.IgnoreRoomLimitAttribute: "false"}

		params := ValidateConnectRequestParams{
			attributes: map[string]string{
				rtc.IgnoreRoomLimitAttribute: "true",
				"role":                       "host",
			},
		}
		res, code, err := ValidateConnectRequest(logger.GetLogger(), grantRequest(g), config.LimitConfig{}, params, nil, stubRoomAllocator{})
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, code)
		require.Equal(t, "false", res.grants.Attributes[rtc.IgnoreRoomLimitAttribute])
		require.False(t, rtc.HasIgnoreRoomLimit(res.grants))
		// other attributes still merge as before
		require.Equal(t, "host", res.grants.Attributes["role"])
	})

	t.Run("client cannot add it to a token without it", func(t *testing.T) {
		g := baseGrant()
		g.Video.SetCanUpdateOwnMetadata(true)

		params := ValidateConnectRequestParams{attributes: map[string]string{rtc.IgnoreRoomLimitAttribute: "true"}}
		res, code, err := ValidateConnectRequest(logger.GetLogger(), grantRequest(g), config.LimitConfig{}, params, nil, stubRoomAllocator{})
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, code)
		require.NotContains(t, res.grants.Attributes, rtc.IgnoreRoomLimitAttribute)
		require.False(t, rtc.HasIgnoreRoomLimit(res.grants))
	})

	t.Run("token value is preserved", func(t *testing.T) {
		g := baseGrant()
		g.Attributes = map[string]string{rtc.IgnoreRoomLimitAttribute: "true"}

		res, code, err := ValidateConnectRequest(logger.GetLogger(), grantRequest(g), config.LimitConfig{}, ValidateConnectRequestParams{}, nil, stubRoomAllocator{})
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, code)
		require.True(t, rtc.HasIgnoreRoomLimit(res.grants))
	})
}
