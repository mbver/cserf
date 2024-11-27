package serf

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

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
