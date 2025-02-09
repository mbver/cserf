// Copyright (c) HashiCorp, Inc.
// Copyright (c) 2024 Phuoc Phi
// SPDX-License-Identifier: MPL-2.0
package serf

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBroadcast_MaxQueueLen(t *testing.T) {
	b := newBroadcastManager(
		func() int { return 100 },
		4,
		4096,
		0,
	)
	require.Equal(t, 4096, b.maxQueueLen())

	b.minQueueDepth = 1024
	require.Equal(t, 1024, b.maxQueueLen())

	b.minQueueDepth = 16
	require.Equal(t, 200, b.maxQueueLen())

	b.numNodes = func() int { return 101 }
	require.Equal(t, 202, b.maxQueueLen())
}
