package serf

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	memberlist "github.com/mbver/mlist"
	"github.com/stretchr/testify/require"
)

func TestIsEventFilterValid(t *testing.T) {
	cases := []struct {
		eventType string
		valid     bool
	}{
		{"member-join", true},
		{"member-leave", true},
		{"member-failed", true},
		{"member-reap", true},
		{"action", true},
		{"Action", false},
		{"member", false},
		{"query", true},
		{"Query", false},
		{"*", true},
	}
	for _, c := range cases {
		require.Equal(
			t,
			c.valid,
			isValidEventFilter(&eventFilter{eventType: c.eventType}),
			fmt.Sprintf("expect %s to be valid: %t", c.eventType, c.valid),
		)
	}
}

func TestParseEventFilter(t *testing.T) {
	cases := []struct {
		in     string
		result []*eventFilter
	}{
		{"", []*eventFilter{{"*", ""}}},
		{"member-join", []*eventFilter{{"member-join", ""}}},
		{"member-reap", []*eventFilter{{"member-reap", ""}}},
		{"foo,bar", []*eventFilter{}}, // these types of events are invalid
		{"action:deploy", []*eventFilter{{"action", "deploy"}}},
		{"query:load", []*eventFilter{{"query", "load"}}},
		{"action:deploy,query:load", []*eventFilter{{"action", "deploy"}, {"query", "load"}}},
		{"member-failed,query:load", []*eventFilter{{"member-failed", ""}, {"query", "load"}}},
	}

	for _, c := range cases {
		res := ParseEventFilters(c.in)
		require.True(
			t,
			reflect.DeepEqual(res, c.result),
			fmt.Sprintf("expect %+v, got: %+v", printSlice(c.result), printSlice(res)),
		)
	}
}

func printSlice(s ...interface{}) string {
	buf := strings.Builder{}
	buf.WriteByte('[')
	for _, f := range s {
		buf.WriteString(fmt.Sprintf("%+v", f))
		buf.WriteByte(',')
	}
	buf.WriteString("]\n")
	return buf.String()
}

func TestCreateScriptHandlers(t *testing.T) {
	cases := []struct {
		s       string
		err     bool
		handers []*ScriptEventHandler
	}{
		{"script.sh", false, []*ScriptEventHandler{{"script.sh", &eventFilter{"*", ""}, nil}}},
		{"member-join=script.sh", false, []*ScriptEventHandler{{"script.sh", &eventFilter{"member-join", ""}, nil}}},
		{"foo,bar=script.sh", false, []*ScriptEventHandler{}},
		{"action:deploy=script.sh", false, []*ScriptEventHandler{{"script.sh", &eventFilter{"action", "deploy"}, nil}}},
		{"query:load=script.sh", false, []*ScriptEventHandler{{"script.sh", &eventFilter{"query", "load"}, nil}}},
		{"query=script.sh", false, []*ScriptEventHandler{{"script.sh", &eventFilter{"query", ""}, nil}}},
		{
			"action:deploy,query:uptime=script.sh",
			false,
			[]*ScriptEventHandler{
				{"script.sh", &eventFilter{"action", "deploy"}, nil},
				{"script.sh", &eventFilter{"query", "uptime"}, nil},
			},
		},
	}

	for _, c := range cases {
		handers := CreateScriptHandlers(c.s, nil)
		require.True(
			t,
			reflect.DeepEqual(handers, c.handers),
			fmt.Sprintf("expect: %s, got: %s", printSlice(c.handers), printSlice(handers)),
		)
	}
}

func TestEventFilter_MatchEvent(t *testing.T) {
	cases := []struct {
		filter *eventFilter
		event  Event
		match  bool
	}{
		{&eventFilter{"*", ""}, &CoalescedMemberEvent{}, true},
		{&eventFilter{"action", ""}, &CoalescedMemberEvent{}, false},
		{&eventFilter{"action", "deploy"}, &ActionEvent{Name: "deploy"}, true},
		{&eventFilter{"member-join", ""}, &CoalescedMemberEvent{Type: EventMemberJoin}, true},
		{&eventFilter{"member-join", ""}, &CoalescedMemberEvent{Type: EventMemberLeave}, false},
		{&eventFilter{"member-reap", ""}, &CoalescedMemberEvent{Type: EventMemberReap}, true},
		{&eventFilter{"query", "deploy"}, &QueryEvent{Name: "deploy"}, true},
		{&eventFilter{"query", "uptime"}, &QueryEvent{Name: "deploy"}, false},
		{&eventFilter{"query", ""}, &QueryEvent{Name: "deploy"}, true},
	}
	for _, c := range cases {
		require.Equal(
			t,
			c.match,
			c.filter.matches(c.event),
			fmt.Sprintf("expect %+v to match %+v: %t", c.filter, c.event, c.match),
		)
	}
}

func createEventRecordScript(scriptfile, recordfile string, script string) error {
	script = fmt.Sprintf(script, recordfile)
	fh, err := os.OpenFile(scriptfile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0755)
	if err != nil {
		return err
	}
	defer fh.Close()
	_, err = fh.Write([]byte(script))
	if err != nil {
		return err
	}
	fh2, err := os.Create(recordfile)
	if err != nil {
		return err
	}
	if err := fh2.Close(); err != nil {
		return err
	}
	return fh.Close()
}

const queryRecordScript = `#!/bin/sh
echo hello
RESULT_FILE="%s"
echo $SERF_LOCAL_ID $SERF_LOCAL_ROLE >>${RESULT_FILE}
echo $SERF_TAG_DC >> ${RESULT_FILE}
echo $SERF_EVENT $SERF_QUERY_NAME "$@" >>${RESULT_FILE}
echo $SERF_EVENT $SERF_QUERY_LTIME "$@" >>${RESULT_FILE}
while read line; do
	printf "${line}\n" >>${RESULT_FILE}
done
`

func TestScriptHandler_QueryEvent(t *testing.T) {
	scriptfile := "./query_record_event.sh"
	recordfile := "query_record.record"
	defer os.Remove(scriptfile)
	defer os.Remove(recordfile)

	err := createEventRecordScript(scriptfile, recordfile, queryRecordScript)
	require.Nil(t, err)

	s, cleanup, err := testNode(&testNodeOpts{
		script: fmt.Sprintf("query=%s", scriptfile),
		tags:   map[string]string{"role": "ourrole", "dc": "east-aws"},
	})
	defer cleanup()
	require.Nil(t, err)

	resCh := make(chan *QueryResponse, 1)
	err = s.Query(resCh, &QueryParam{
		Name:    "uptime",
		Payload: []byte("load average"),
		Timeout: 400 * time.Millisecond, // this script take a lot of time due to I/O
	})
	require.Nil(t, err)
	select {
	case <-resCh:
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("query timeout")
	}

	record, err := os.ReadFile(recordfile)
	require.Nil(t, err)
	expect := fmt.Sprintf(`%s ourrole
east-aws
query uptime
query 1
load average
`, s.ID())
	require.Equal(t, expect, string(record), fmt.Sprintf("expect:%s, got:%s", expect, record))
}

const actionScript = `#!/bin/sh
RESULT_FILE="%s"
echo $SERF_LOCAL_ID $SERF_LOCAL_ROLE >>${RESULT_FILE}
echo $SERF_TAG_DC >> ${RESULT_FILE}
echo $SERF_EVENT $SERF_ACTION_EVENT "$@" >>${RESULT_FILE}
echo $SERF_EVENT $SERF_ACTION_LTIME "$@" >>${RESULT_FILE}
while read line; do
	printf "${line}\n" >>${RESULT_FILE}
done
`

func TestScriptHandler_ActionEvent(t *testing.T) {
	scriptfile := "./action_record_event.sh"
	recordfile := "action_record.record"
	defer os.Remove(scriptfile)
	defer os.Remove(recordfile)

	err := createEventRecordScript(scriptfile, recordfile, actionScript)
	require.Nil(t, err)

	s, cleanup, err := testNode(&testNodeOpts{
		script: fmt.Sprintf("action=%s", scriptfile),
		tags:   map[string]string{"role": "ourrole", "dc": "east-aws"},
	})
	defer cleanup()
	require.Nil(t, err)

	s.Action("deploy", []byte("that thing"))
	var output []byte

	expect := fmt.Sprintf(`%s ourrole
east-aws
action deploy
action 1
that thing
`, s.ID())
	success, msg := retry(5, func() (bool, string) {
		time.Sleep(100 * time.Millisecond)
		output, err = os.ReadFile(recordfile)
		if string(output) != expect {
			return false, fmt.Sprintf("got %s", output)
		}
		return true, ""
	})
	require.True(t, success, msg)
}

const memberScript = `#!/bin/sh
RESULT_FILE="%s"
echo $SERF_LOCAL_ID $SERF_LOCAL_ROLE >>${RESULT_FILE}
echo $SERF_TAG_DC >> ${RESULT_FILE}
echo $SERF_TAG_BAD_TAG >> ${RESULT_FILE}
echo $SERF_EVENT >>${RESULT_FILE}
while read line; do
	printf "${line}\n" >>${RESULT_FILE}
done
`

func TestScriptHandler_MemberEvent(t *testing.T) {
	scriptfile := "./member_record_event.sh"
	recordfile := "member_record.record"
	defer os.Remove(scriptfile)
	defer os.Remove(recordfile)

	err := createEventRecordScript(scriptfile, recordfile, memberScript)
	require.Nil(t, err)

	s, cleanup, err := testNode(&testNodeOpts{
		script: fmt.Sprintf("*=%s", scriptfile),
		tags:   map[string]string{"role": "ourrole", "dc": "east-aws", "bad-tag": "bad"},
	})
	defer cleanup()
	require.Nil(t, err)

	tags := map[string]string{"role": "balancer", "job": "off-balance"}
	encoded, err := EncodeTags(tags)
	require.Nil(t, err)
	s.inEventCh <- &MemberEvent{
		Type: EventMemberJoin,
		Member: &memberlist.Node{
			ID:   "foo",
			IP:   []byte{1, 2, 3, 4},
			Port: 6555,
			Tags: encoded,
		},
	}

	var output []byte
	ip, _, err := s.AdvertiseIpPort()
	require.Nil(t, err)
	part1 := fmt.Sprintf(`%s ourrole
east-aws
bad
member-join`, s.ID())

	part2 := "foo	1.2.3.4	balancer	job=off-balance,role=balancer"
	part3 := fmt.Sprintf("%s	%s	ourrole	bad-tag=bad,dc=east-aws,role=ourrole", s.ID(), ip.String())
	success, msg := retry(5, func() (bool, string) {
		time.Sleep(100 * time.Millisecond)
		output, err = os.ReadFile(recordfile)
		s := string(output)
		if strings.Contains(s, part1) && strings.Contains(s, part2) && strings.Contains(s, part3) {
			return true, ""
		}
		return false, fmt.Sprintf("got: %s", s)
	})
	require.True(t, success, msg)
}
