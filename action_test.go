package serf

import (
	"bytes"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func checkActions(ch chan Event, names []string, payloads [][]byte) (bool, string) {
	n := len(ch)
	aEvents := []*ActionEvent{}
	for i := 0; i < n; i++ {
		e := <-ch
		a, ok := e.(*ActionEvent)
		if !ok {
			continue
		}
		aEvents = append(aEvents, a)
	}
	if len(names) != len(aEvents) {
		return false, fmt.Sprintf("mismatch number of events: expect %d, got %d", len(names), len(aEvents))
	}
	for i, a := range aEvents {
		if names[i] != a.Name {
			return false, "mismatch name: " + a.Name
		}
		if !bytes.Equal(payloads[i], a.Payload) {
			return false, "mismatch payload: " + string(a.Payload)
		}
	}
	return true, ""
}

func TestSerf_Action(t *testing.T) {
	eventCh := make(chan Event, 10)
	_, s2, cleanup, err := twoNodesJoinedWithEventStream(eventCh)
	defer cleanup()
	require.Nil(t, err)

	err = s2.Action("first", []byte("first-test"))
	require.Nil(t, err)

	err = s2.Action("second", []byte("second-test"))
	require.Nil(t, err)

	enough, msg := retry(5, func() (bool, string) {
		time.Sleep(50 * time.Millisecond)
		if len(eventCh) != 3 {
			return false, "not enough events"
		}
		return true, ""
	})
	require.True(t, enough, msg)

	match, msg := checkActions(eventCh,
		[]string{"first", "second"},
		[][]byte{[]byte("first-test"), []byte("second-test")})
	require.True(t, match, msg)
}

func TestSerf_Action_SizeLimit(t *testing.T) {
	s, cleanup, err := testNode(nil)
	defer cleanup()
	require.Nil(t, err)

	payload := make([]byte, s.config.ActionSizeLimit)
	err = s.Action("big action", payload)
	require.NotNil(t, err)
	require.True(t, errors.Is(err, ErrActionSizeLimitExceed))
}

func TestSerf_Action_OldMsg(t *testing.T) {
	s, cleanup, err := testNode(nil)
	defer cleanup()
	require.Nil(t, err)

	s.action.clock.Witness(LamportTime(s.config.LBufferSize + 1000))
	msg := msgAction{
		LTime:   1,
		Name:    "old",
		Payload: nil,
	}
	require.False(t, s.action.addToBuffer(&msg))
}

func TestSerf_Action_SameClock(t *testing.T) {
	s, cleanup, err := testNode(nil)
	defer cleanup()
	require.Nil(t, err)

	msg := msgAction{
		LTime: 1,
		ID:    1,
		Name:  "first",
	}
	require.True(t, s.action.addToBuffer(&msg), "should be added")

	msg.ID = 2
	require.True(t, s.action.addToBuffer(&msg), "should be added")

	msg.ID = 3
	require.True(t, s.action.addToBuffer(&msg), "should be added")
}
