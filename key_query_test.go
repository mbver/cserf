package serf

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	memberlist "github.com/mbver/mlist"
	"github.com/stretchr/testify/require"
)

func TestSerf_WriteKeyringFile(t *testing.T) {
	existing := "T9jncgl9mbLus+baTTa7q7nPSUrXwbDi2dhbtqir37s="
	newKey := "HvY8ubRZMgafUOWvrOadwOckVa1wN3QWAo46FVKbVN8="

	td, err := os.MkdirTemp("", "serf")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.RemoveAll(td)

	s1, cleanup, err := testNode(nil)
	defer cleanup()
	require.Nil(t, err)

	keyringFile := filepath.Join(td, "keys.json")
	s1.config.KeyringFile = keyringFile

	resp, err := s1.KeyQuery("install", newKey)
	require.Nil(t, err)
	require.Equal(t, 1, resp.NumNode)
	require.Equal(t, 1, resp.NumResp)
	require.Equal(t, 0, resp.NumErr)
	require.Equal(t, 0, len(resp.ErrFrom))

	content, err := os.ReadFile(keyringFile)
	require.Nil(t, err)

	lines := strings.Split(string(content), "\n")
	require.Equal(t, 4, len(lines))

	// ensure both the original key and the new key are present in the file
	require.Contains(t, string(content), existing)
	require.Contains(t, string(content), newKey)

	// change primary key
	resp, err = s1.KeyQuery("use", newKey)
	require.Nil(t, err)
	require.Equal(t, 1, resp.NumNode)
	require.Equal(t, 1, resp.NumResp)
	require.Equal(t, 0, resp.NumErr)
	require.Equal(t, 0, len(resp.ErrFrom))

	content, err = os.ReadFile(keyringFile)
	require.Nil(t, err)

	lines = strings.Split(string(content), "\n")
	require.Equal(t, 4, len(lines))

	// Ensure both the original key and the new key are present in the file
	require.Contains(t, string(content), existing)
	require.Contains(t, string(content), newKey)

	// remove the old key
	resp, err = s1.KeyQuery("remove", existing)
	require.Nil(t, err)
	require.Equal(t, 1, resp.NumNode)
	require.Equal(t, 1, resp.NumResp)
	require.Equal(t, 0, resp.NumErr)
	require.Equal(t, 0, len(resp.ErrFrom))

	content, err = os.ReadFile(keyringFile)
	require.Nil(t, err)

	lines = strings.Split(string(content), "\n")
	require.Equal(t, 3, len(lines))

	// ensure only the new key are present in the file
	require.NotContains(t, string(content), existing)
	require.Contains(t, string(content), newKey)
}

func testNodeWithKeyring() (*Serf, func(), error) {
	cleanup := func() {}
	keysEncoded := []string{
		"ZWTL+bgjHyQPhJRKcFe3ccirc2SFHmc/Nw67l8NQfdk=",
		"WbL6oaTPom+7RG7Q/INbJWKy09OLar/Hf2SuOAdoQE4=",
		"HvY8ubRZMgafUOWvrOadwOckVa1wN3QWAo46FVKbVN8=",
	}
	keys := make([][]byte, len(keysEncoded))
	for i, key := range keysEncoded {
		decoded, err := base64.StdEncoding.DecodeString(key)
		if err != nil {
			return nil, cleanup, err
		}
		keys[i] = decoded
	}
	kr, err := memberlist.NewKeyring(keys, keys[0])
	if err != nil {
		return nil, cleanup, err
	}
	return testNode(&testNodeOpts{
		keyring: kr,
	})
}

func twoNodesJoinedWithKeyring() (*Serf, *Serf, func(), error) {
	s1, cleanup1, err := testNodeWithKeyring()
	if err != nil {
		return nil, nil, cleanup1, err
	}
	s2, cleanup2, err := testNodeWithKeyring()
	cleanup := combineCleanup(cleanup1, cleanup2)
	if err != nil {
		return nil, nil, cleanup, err
	}
	addr, err := s2.AdvertiseAddress()
	if err != nil {
		return nil, nil, cleanup, err
	}
	n, err := s1.Join([]string{addr}, false)
	if err != nil {
		return nil, nil, cleanup, err
	}
	if n != 1 {
		return nil, nil, cleanup, fmt.Errorf("unsuccessful join")
	}
	return s1, s2, cleanup, err
}

func ringHasKey(kr *memberlist.Keyring, keyEnc string) bool {
	key, err := base64.StdEncoding.DecodeString(keyEnc)
	if err != nil {
		return false
	}
	for _, k := range kr.GetKeys() {
		if bytes.Equal(key, k) {
			return true
		}
	}
	return false
}
func TestSerf_InstallKey(t *testing.T) {
	s1, s2, cleanup, err := twoNodesJoinedWithKeyring()
	defer cleanup()
	require.Nil(t, err)
	primaryKey := s1.keyring.GetPrimaryKey()

	key := "T9jncgl9mbLus+baTTa7q7nPSUrXwbDi2dhbtqir37s="

	resp, err := s1.KeyQuery("install", key)
	require.Nil(t, err)
	require.Equal(t, 2, resp.NumNode)
	require.Equal(t, 2, resp.NumResp)
	require.Zero(t, resp.NumErr)
	require.Zero(t, len(resp.ErrFrom))

	// primary key not changed
	require.True(t, bytes.Equal(primaryKey, s1.keyring.GetPrimaryKey()))
	require.True(t, bytes.Equal(primaryKey, s2.keyring.GetPrimaryKey()))

	require.True(t, ringHasKey(s1.keyring, key))
	require.True(t, ringHasKey(s2.keyring, key))
}

func TestSerf_UseKey(t *testing.T) {
	s1, s2, cleanup, err := twoNodesJoinedWithKeyring()
	defer cleanup()
	require.Nil(t, err)

	key := "HvY8ubRZMgafUOWvrOadwOckVa1wN3QWAo46FVKbVN8="
	resp, err := s1.KeyQuery("use", key)
	require.Nil(t, err)
	require.Equal(t, 2, resp.NumNode)
	require.Equal(t, 2, resp.NumResp)
	require.Zero(t, resp.NumErr)
	require.Zero(t, len(resp.ErrFrom))

	// primary key changed
	keyDec, err := base64.StdEncoding.DecodeString(key)
	require.Nil(t, err)
	require.True(t, bytes.Equal(keyDec, s1.keyring.GetPrimaryKey()))
	require.True(t, bytes.Equal(keyDec, s2.keyring.GetPrimaryKey()))
}

func TestSerf_UseKey_Rejected(t *testing.T) {
	s1, s2, cleanup, err := twoNodesJoinedWithKeyring()
	defer cleanup()
	require.Nil(t, err)
	primaryKey := s1.keyring.GetPrimaryKey()

	key := "T9jncgl9mbLus+baTTa7q7nPSUrXwbDi2dhbtqir37s=" // non-existent
	resp, err := s1.KeyQuery("use", key)
	require.Nil(t, err)
	require.Equal(t, 2, resp.NumNode)
	require.Equal(t, 2, resp.NumResp)
	require.Equal(t, 2, resp.NumErr)
	require.Equal(t, 2, len(resp.ErrFrom))
	for _, msg := range resp.ErrFrom {
		require.Contains(t, msg, "not in keyring")
	}
	// primary key not changed
	require.True(t, bytes.Equal(primaryKey, s1.keyring.GetPrimaryKey()))
	require.True(t, bytes.Equal(primaryKey, s2.keyring.GetPrimaryKey()))
}

func TestSerf_RemoveKey(t *testing.T) {
	s1, s2, cleanup, err := twoNodesJoinedWithKeyring()
	defer cleanup()
	require.Nil(t, err)
	primaryKey := s1.keyring.GetPrimaryKey()

	key := "HvY8ubRZMgafUOWvrOadwOckVa1wN3QWAo46FVKbVN8="
	resp, err := s1.KeyQuery("remove", key)
	require.Nil(t, err)
	require.Equal(t, 2, resp.NumNode)
	require.Equal(t, 2, resp.NumResp)
	require.Zero(t, resp.NumErr)
	require.Zero(t, len(resp.ErrFrom))

	// primary key notchanged
	require.True(t, bytes.Equal(primaryKey, s1.keyring.GetPrimaryKey()))
	require.True(t, bytes.Equal(primaryKey, s2.keyring.GetPrimaryKey()))

	// key is deleted
	require.False(t, ringHasKey(s1.keyring, key))
	require.False(t, ringHasKey(s2.keyring, key))
}

func TestSerf_RemoveKey_Reject_Primary(t *testing.T) {
	s1, s2, cleanup, err := twoNodesJoinedWithKeyring()
	defer cleanup()
	require.Nil(t, err)
	primaryKey := s1.keyring.GetPrimaryKey()

	key := "ZWTL+bgjHyQPhJRKcFe3ccirc2SFHmc/Nw67l8NQfdk=" // primary key
	resp, err := s1.KeyQuery("remove", key)
	require.Nil(t, err)
	require.Equal(t, 2, resp.NumNode)
	require.Equal(t, 2, resp.NumResp)
	require.Equal(t, 2, resp.NumErr)
	require.Equal(t, 2, len(resp.ErrFrom))

	// primary key not changed
	require.True(t, bytes.Equal(primaryKey, s1.keyring.GetPrimaryKey()))
	require.True(t, bytes.Equal(primaryKey, s2.keyring.GetPrimaryKey()))

	// key is not deleted
	require.True(t, ringHasKey(s1.keyring, key))
	require.True(t, ringHasKey(s2.keyring, key))
}

func TestSerf_RemoveKey_Reject_NonExist(t *testing.T) {
	s1, s2, cleanup, err := twoNodesJoinedWithKeyring()
	defer cleanup()
	require.Nil(t, err)
	primaryKey := s1.keyring.GetPrimaryKey()

	key := "T9jncgl9mbLus+baTTa7q7nPSUrXwbDi2dhbtqir37s=" // non-existent
	resp, err := s1.KeyQuery("remove", key)
	require.Nil(t, err)
	require.Equal(t, 2, resp.NumNode)
	require.Equal(t, 2, resp.NumResp)
	require.Equal(t, 2, resp.NumErr)
	require.Equal(t, 2, len(resp.ErrFrom))

	// primary key notchanged
	require.True(t, bytes.Equal(primaryKey, s1.keyring.GetPrimaryKey()))
	require.True(t, bytes.Equal(primaryKey, s2.keyring.GetPrimaryKey()))

	// key is deleted
	require.False(t, ringHasKey(s1.keyring, key))
	require.False(t, ringHasKey(s2.keyring, key))
}

func TestSerf_ListKey(t *testing.T) {
	s1, s2, cleanup, err := twoNodesJoinedWithKeyring()
	defer cleanup()
	require.Nil(t, err)
	primaryKey := s1.keyring.GetPrimaryKey()
	require.Equal(t, 3, len(s1.keyring.GetKeys()))

	keyEnc := "5K9OtfP7efFrNKe5WCQvXvnaXJ5cWP0SvXiwe0kkjM4="
	key, err := base64.StdEncoding.DecodeString(keyEnc)
	require.Nil(t, err)
	err = s2.keyring.AddKey(key)
	require.Nil(t, err)

	resp, err := s1.KeyQuery("list", "")
	require.Nil(t, err)
	require.Equal(t, 2, resp.NumNode)
	require.Equal(t, 2, resp.NumResp)
	require.Zero(t, resp.NumErr)
	require.Zero(t, len(resp.ErrFrom))

	require.Equal(t, 1, len(resp.PrimaryKeyCount))
	enc := base64.StdEncoding.EncodeToString(primaryKey)
	require.Equal(t, 2, resp.PrimaryKeyCount[enc])

	require.Equal(t, 4, len(resp.KeyCount))
	for k, v := range resp.KeyCount {
		if k == keyEnc {
			require.Equal(t, 1, v, fmt.Sprintf("got %d", v))
			continue
		}
		require.Equal(t, 2, v, fmt.Sprintf("got %d", v))
	}
}

func genTestKeys(n int) []string {
	res := make([]string, n)
	for i := 0; i < n; i++ {
		key := make([]byte, 32)
		for j := 0; j < 32; j++ {
			key[j] = byte(randIntN(256))
		}
		res[i] = base64.StdEncoding.EncodeToString(key)
	}
	return res
}

func TestSerf_ListKey_Truncated(t *testing.T) {
	s1, s2, cleanup, err := twoNodesJoined()
	defer cleanup()
	require.Nil(t, err)

	cases := []struct {
		keys     []string
		expected int
		err      bool
	}{
		{
			[]string{"KfCPZAKdgHUOdb202afZfE8EbdZqj4+ReTbfJUkfKsg="},
			2,
			false,
		},
		{
			genTestKeys(50),
			18,
			true,
		},
		{
			genTestKeys(40),
			18,
			true,
		},
		{
			genTestKeys(18),
			18,
			true,
		},
		{
			genTestKeys(15),
			16,
			false,
		},
	}

	for _, c := range cases {
		addedKeys := make([][]byte, len(c.keys))
		for i, keyEnc := range c.keys {
			k, err := base64.StdEncoding.DecodeString(keyEnc)
			require.Nil(t, err)
			err = s2.keyring.AddKey(k)
			addedKeys[i] = k
			require.Nil(t, err)
		}

		resp, err := s1.KeyQuery("list", "")
		require.Nil(t, err)
		require.Equal(t, 2, resp.NumNode)
		require.Equal(t, 2, resp.NumResp)
		require.Equal(t, !c.err, len(resp.ErrFrom) == 0)
		if len(resp.ErrFrom) != 0 {
			require.Contains(t, resp.ErrFrom[s2.ID()], "truncated")
		}
		require.Equal(t, c.expected, len(resp.KeyCount))

		// cleanup for next test
		for _, k := range addedKeys {
			err = s2.keyring.RemoveKey(k)
			require.Nil(t, err)
		}
	}
}

func TestKeyQueryReceptor_PassThrough(t *testing.T) {
	eventCh := make(chan Event, 10)
	s, cleanup, err := testNode(&testNodeOpts{eventCh: eventCh})
	defer cleanup()
	require.Nil(t, err)
	s.inEventCh <- &QueryEvent{LTime: 10, Name: keyCommandToQueryName("install")}
	s.inEventCh <- &QueryEvent{LTime: 11, Name: keyCommandToQueryName("use")}
	s.inEventCh <- &QueryEvent{LTime: 12, Name: keyCommandToQueryName("remove")}

	aEvent := &ActionEvent{LTime: 42, Name: "foo"}
	s.inEventCh <- aEvent

	qEvent := &QueryEvent{LTime: 42, Name: "foo"}
	s.inEventCh <- qEvent

	mEvent := &MemberEvent{
		Type:   EventMemberJoin,
		Member: &memberlist.Node{}}
	s.inEventCh <- mEvent

	enough, msg := retry(5, func() (bool, string) {
		time.Sleep(20 * time.Millisecond)
		if len(eventCh) != 3 {
			return false, fmt.Sprintf("missing events: %d/3", len(eventCh))
		}
		return true, ""
	})
	require.True(t, enough, msg)

	cEvent := &CoalescedMemberEvent{
		Type:    mEvent.EventType(),
		Members: []*memberlist.Node{mEvent.Member},
	}
	for _, event := range []Event{aEvent, qEvent, cEvent} {
		e := <-eventCh
		require.True(t,
			reflect.DeepEqual(e, event),
			fmt.Sprintf("got: %+v, expect: %+v", e, event),
		)
	}
}
