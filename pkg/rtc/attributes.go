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
	"strings"

	"github.com/livekit/protocol/auth"
)

// IgnoreRoomLimitAttribute exempts a participant from the room's
// max_participants limit: it may join a full room, and it is not counted
// towards the limit.
//
// The exemption is honored only when it comes from the signed access token.
// Attributes sent by the client on connect, or updated by the participant
// later, are ignored (see service.ValidateConnectRequest) so a participant
// cannot grant itself a slot.
const IgnoreRoomLimitAttribute = "ignore_room_limit"

// HasIgnoreRoomLimit reports whether the participant's signed grants carry the
// room-limit exemption. Any value other than "false"/"0"/"no"/"off" (including
// an empty value) enables it, so setting the attribute is enough.
func HasIgnoreRoomLimit(grants *auth.ClaimGrants) bool {
	if grants == nil {
		return false
	}

	value, ok := grants.Attributes[IgnoreRoomLimitAttribute]
	if !ok {
		return false
	}

	switch strings.ToLower(strings.TrimSpace(value)) {
	case "false", "0", "no", "off":
		return false
	default:
		return true
	}
}

// ignoresRoomLimit reports whether the participant joined with the room-limit
// exemption. The decision is frozen at join time in ParticipantOptions.
func ignoresRoomLimit(opts *ParticipantOptions) bool {
	return opts != nil && opts.IgnoreRoomLimit
}
