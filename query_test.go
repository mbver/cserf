package serf

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func threeNodesJoined() (*Serf, *Serf, *Serf, func(), error) {
	s1, s2, s3, cleanup, err := threeNodes()
	if err != nil {
		return nil, nil, nil, cleanup, err
	}
	addr2, err := s2.AdvertiseAddress()
	if err != nil {
		return nil, nil, nil, cleanup, err
	}
	addr3, err := s3.AdvertiseAddress()
	if err != nil {
		return nil, nil, nil, cleanup, err
	}

	n, err := s1.Join([]string{addr2, addr3}, false)
	if n != 2 {
		return nil, nil, nil, cleanup, fmt.Errorf("missing in join")
	}
	if err != nil {
		return nil, nil, nil, cleanup, err
	}
	return s1, s2, s3, cleanup, nil
}

// The chance of failing this test is that a node is not receiving broadcast msg from two nodes.
// Each node broadcast msg 4 times ==> (0.5)^(2*4) = 0.39%
// That is 1 in each 250 runs!
func TestSerf_Query(t *testing.T) {
	s1, s2, s3, cleanup, err := threeNodesJoined()
	defer cleanup()
	require.Nil(t, err)

	respCh := make(chan *QueryResponse, 3)
	s1.Query(respCh, nil)

	success, msg := retry(5, func() (bool, string) {
		time.Sleep(10 * time.Millisecond)
		if len(respCh) != 3 {
			return false, fmt.Sprintf("receive only %d/3", len(respCh))
		}
		found := make([]bool, 3)
		for i := 0; i < 3; i++ {
			r := <-respCh
			for i, s := range []*Serf{s1, s2, s3} {
				if r.From == s.ID() {
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

func TestSerf_Query_SizeLimit(t *testing.T) {
	s, cleanup, err := testNode(nil)
	defer cleanup()
	require.Nil(t, err)

	payload := make([]byte, s.config.QuerySizeLimit)
	params := s.DefaultQueryParams()
	params.Payload = payload
	err = s.Query(make(chan *QueryResponse), params)
	require.NotNil(t, err)
	require.True(t, errors.Is(err, ErrQuerySizeLimitExceed))
}

func TestSerf_Query_SizeLimit_Increased(t *testing.T) {
	s, cleanup, err := testNode(nil)
	defer cleanup()
	require.Nil(t, err)

	payload := make([]byte, s.config.QuerySizeLimit)
	params := s.DefaultQueryParams()
	params.Payload = payload
	s.config.QuerySizeLimit = 2048
	err = s.Query(make(chan *QueryResponse, 1), params)
	require.Nil(t, err)
}

func TestSerf_Query_FilterNodes(t *testing.T) {
	s1, s2, _, cleanup, err := threeNodesJoined()
	defer cleanup()
	require.Nil(t, err)

	params := s2.DefaultQueryParams()
	params.ForNodes = []string{s1.ID()}

	respCh := make(chan *QueryResponse, 3)
	s1.Query(respCh, params)

	success, msg := retry(5, func() (bool, string) {
		time.Sleep(10 * time.Millisecond)
		if len(respCh) != 1 {
			return false, fmt.Sprintf("receive %d/1", len(respCh))
		}
		return true, ""
	})
	require.True(t, success, msg)
	e := <-respCh
	require.Equal(t, s1.ID(), e.From)
}

func TestSerf_Query_Duplicate(t *testing.T) {
	s, cleanup, err := testNode(nil)
	defer cleanup()
	require.Nil(t, err)
	respCh := make(chan *QueryResponse, 3)
	s.query.setResponseHandler(3, 123, respCh, 10*time.Second)

	ip, port, err := s.mlist.GetAdvertiseAddr()
	require.Nil(t, err)
	msg := msgQuery{
		LTime:      3,
		ID:         123,
		SourceIP:   ip,
		SourcePort: port,
	}

	encoded, err := encode(msgQueryType, msg)
	require.Nil(t, err)
	s.handleQuery(encoded)
	s.handleQuery(encoded)

	resMsg := msgQueryResponse{
		LTime: 3,
		ID:    123,
		From:  s.ID(),
	}
	encoded, err = encode(msgQueryRespType, resMsg)
	require.Nil(t, err)
	s.handleQueryResponse(encoded)
	s.query.invokeResponseHandler(&resMsg)

	time.Sleep(50 * time.Millisecond)

	require.Equal(t, 1, len(respCh))
	qResp := <-respCh
	require.Equal(t, s.ID(), qResp.From)
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
