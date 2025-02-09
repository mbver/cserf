// Copyright (c) HashiCorp, Inc.
// Copyright (c) 2024 Phuoc Phi
// SPDX-License-Identifier: MPL-2.0
package serf

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMemberEvent(t *testing.T) {
	eTypes := []EventType{EventMemberJoin, EventMemberLeave, EventMemberFailed, EventMemberUpdate, EventMemberReap}
	eStrings := []string{"member-join", "member-leave", "member-failed", "member-update", "member-reap"}
	mEvent := MemberEvent{}
	for i := 0; i < len(eTypes); i++ {
		mEvent.Type = eTypes[i]
		require.Equal(t, eTypes[i], mEvent.EventType())
		require.Equal(t, eStrings[i], mEvent.String())
	}
}

func TestActionEvent(t *testing.T) {
	aEvent := ActionEvent{
		Name:    "deploy",
		Payload: []byte("something"),
	}
	require.Equal(t, EventAction, aEvent.EventType())
	require.Equal(t, "action-event: deploy", aEvent.String())
}

func TestQueryEvent(t *testing.T) {
	qEvent := QueryEvent{
		LTime:   42,
		Name:    "update",
		Payload: []byte("abcd1234"),
	}
	require.Equal(t, EventQuery, qEvent.EventType())
	require.Equal(t, "query: update", qEvent.String())
}

func TestEventType_String(t *testing.T) {
	eTypes := []EventType{EventMemberJoin, EventMemberLeave, EventMemberFailed,
		EventMemberUpdate, EventMemberReap, EventAction, EventQuery, EventType(100)}
	eStrings := []string{"member-join", "member-leave", "member-failed",
		"member-update", "member-reap", "action", "query", "unknown-event"}

	for i := range eTypes {
		require.Equal(t, eStrings[i], eTypes[i].String())
	}
}
