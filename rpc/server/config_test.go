package server

import (
	"os"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestConfig_Load(t *testing.T) {
	c1 := DefaultServerConfig()
	ybytes, err := yaml.Marshal(c1)
	require.Nil(t, err)

	path := "./testconf.yaml"
	defer os.Remove(path)
	fh, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	require.Nil(t, err)
	_, err = fh.Write(ybytes)
	require.Nil(t, err)

	c2, err := LoadConfig(path)
	require.Nil(t, err)
	require.True(t, reflect.DeepEqual(c1, c2))
}
