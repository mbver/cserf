// Copyright (c) HashiCorp, Inc.
// Copyright (c) 2024 Phuoc Phi
// SPDX-License-Identifier: MPL-2.0
package serf

import (
	"testing"
	"time"

	memberlist "github.com/mbver/mlist"
	"github.com/stretchr/testify/require"
)

func TestInactive_Reap(t *testing.T) {
	in := newInactiveNodes(5*time.Millisecond, 10*time.Millisecond)
	f1 := &memberlist.Node{
		ID: "f1",
	}
	f2 := &memberlist.Node{
		ID: "f2",
	}
	in.addFail(f1)
	in.addFail(f2)
	time.Sleep(6 * time.Millisecond)
	f3 := &memberlist.Node{
		ID: "f3",
	}
	in.addFail(f3)
	require.Equal(t, 3, len(in.failed))
	require.Zero(t, len(in.left))

	reaped := in.reap()
	require.Equal(t, 2, len(reaped))
	require.Equal(t, reaped[0].ID, f1.ID)
	require.Equal(t, reaped[1].ID, f2.ID)
	require.Equal(t, 1, len(in.failed))
	require.Zero(t, len(in.left))

	l1 := &memberlist.Node{
		ID: "l1",
	}
	l2 := &memberlist.Node{
		ID: "l2",
	}
	in.addLeftBatch(l1, l2)
	time.Sleep(11 * time.Millisecond)
	l3 := &memberlist.Node{
		ID: "l3",
	}
	in.addLeft(l3)
	require.Equal(t, 3, len(in.left))
	require.Equal(t, 1, len(in.failed))

	reaped = in.reap()
	require.Equal(t, 3, len(reaped))
	require.Equal(t, reaped[0].ID, f3.ID)
	require.Equal(t, reaped[1].ID, l1.ID)
	require.Equal(t, reaped[2].ID, l2.ID)
	require.Equal(t, 1, len(in.left))
	require.Zero(t, len(in.failed))
}
