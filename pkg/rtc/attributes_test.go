// Copyright 2026 Acme, Inc.
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

package rtc

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/livekit/protocol/auth"
)

func TestHasIgnoreRoomLimit(t *testing.T) {
	tests := []struct {
		name     string
		grants   *auth.ClaimGrants
		expected bool
	}{
		{name: "no grants", grants: nil, expected: false},
		{name: "no attributes", grants: &auth.ClaimGrants{}, expected: false},
		{name: "other attribute", grants: &auth.ClaimGrants{Attributes: map[string]string{"role": "host"}}, expected: false},
		{name: "true", grants: &auth.ClaimGrants{Attributes: map[string]string{IgnoreRoomLimitAttribute: "true"}}, expected: true},
		{name: "uppercase", grants: &auth.ClaimGrants{Attributes: map[string]string{IgnoreRoomLimitAttribute: "TRUE"}}, expected: true},
		{name: "empty value", grants: &auth.ClaimGrants{Attributes: map[string]string{IgnoreRoomLimitAttribute: ""}}, expected: true},
		{name: "one", grants: &auth.ClaimGrants{Attributes: map[string]string{IgnoreRoomLimitAttribute: "1"}}, expected: true},
		{name: "false", grants: &auth.ClaimGrants{Attributes: map[string]string{IgnoreRoomLimitAttribute: "false"}}, expected: false},
		{name: "zero", grants: &auth.ClaimGrants{Attributes: map[string]string{IgnoreRoomLimitAttribute: "0"}}, expected: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expected, HasIgnoreRoomLimit(tc.grants))
		})
	}
}
