package serf

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSerf_Query(t *testing.T) {
	s1, s2, s3, cleanup, err := threeNodes()
	defer cleanup()
	require.Nil(t, err)

	addr2, err := s2.AdvertiseAddress()
	require.Nil(t, err)

	addr3, err := s3.AdvertiseAddress()
	require.Nil(t, err)

	n, err := s1.Join([]string{addr2, addr3})
	require.Nil(t, err)
	require.Equal(t, 2, n)
	respCh := make(chan string, 3)
	s1.Query(respCh, nil)

	success, msg := retry(5, func() (bool, string) {
		time.Sleep(10 * time.Millisecond)
		if len(respCh) != 3 {
			return false, fmt.Sprintf("receive only %d/3", len(respCh))
		}
		found := make([]bool, 3)
		for i := 0; i < 3; i++ {
			str := <-respCh
			for i, s := range []*Serf{s1, s2, s3} {
				if str == s.ID() {
					found[i] = true
				}
			}
		}
		for i, v := range found {
			if !v {
				return false, fmt.Sprintf("missing %d", i)
			}
		}
		return true, ""
	})
	require.True(t, success, msg)
}

func TestSerf_IsQueryAccepted(t *testing.T) {
	tags := map[string]string{
		"role":       "webserver",
		"datacenter": "east-aws",
	}
	s, cleanup, err := testNode(tags)
	defer cleanup()
	require.Nil(t, err)

	cases := []struct {
		nodes      []string
		filtertags []FilterTag
		accepted   bool
	}{
		{
			nodes: []string{"foo", "bar", s.ID()},
			filtertags: []FilterTag{
				{"role", "^web"},
				{"datacenter", "aws$"},
			},
			accepted: true,
		},
		{
			nodes: []string{"foo", "bar"},
			filtertags: []FilterTag{
				{"role", "^web"},
				{"datacenter", "aws$"},
			},
			accepted: false,
		},
		{
			filtertags: []FilterTag{
				{"role", "^web"},
				{"datacenter", "aws$"},
			},
			accepted: true,
		},
		{
			nodes:    []string{"foo", "bar"},
			accepted: false,
		},
		{
			filtertags: []FilterTag{
				{"other", "cool"},
			},
			accepted: false,
		},
		{
			filtertags: []FilterTag{
				{"role", "db"},
			},
			accepted: false,
		},
	}

	for _, c := range cases {
		q := &msgQuery{
			ForNodes:   c.nodes,
			FilterTags: c.filtertags,
		}
		accepted := s.isQueryAccepted(q)
		if accepted != c.accepted {
			t.Errorf("result for %+v not matched. expect %t, got %t", q, c.accepted, accepted)
		}
	}
}
