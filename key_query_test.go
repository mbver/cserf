package serf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

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
