package main

import (
	"path/filepath"

	"github.com/mbver/cserf/cmd/utils"
	"github.com/spf13/cobra"
)

func CertGenCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "cert <dir>",
		Short: "generate self-signed cert and key, for testing purpose",
		Run: func(cmd *cobra.Command, args []string) {
			out := utils.CreateOutputFromCmd(cmd)
			if len(args) < 1 {
				out.Error(ErrAtLeastOneArg)
				return
			}
			dir := args[0]
			certfile := filepath.Join(dir, "cert.pem") // SUFFIX?
			keyfile := filepath.Join(dir, "priv.key")  // WHAT SUFFIX? PEM?
			err := utils.GenerateSelfSignedCert(certfile, keyfile)
			if err != nil {
				out.Error(err)
				return
			}
			out.Result("successfully genrate cert and key in", dir)
		},
	}
}
