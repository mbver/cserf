package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"github.com/mbver/cserf/cmd/utils"
	"github.com/spf13/cobra"
)

func KeyGenCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "keygen",
		Short: "generate a 32-bytes key",
		Run: func(cmd *cobra.Command, args []string) {
			out := utils.CreateOutputFromCmd(cmd)
			key := make([]byte, 32)
			n, err := rand.Reader.Read(key)
			if err != nil {
				out.Error(err)
				return
			}
			if n != 32 {
				out.Error(fmt.Errorf("couldn't read enough random bytes: %d/%d", n, 32))
				return
			}
			out.Result("key", base64.StdEncoding.EncodeToString(key))
		},
	}
}
