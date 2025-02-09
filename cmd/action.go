// Copyright (c) HashiCorp, Inc.
// Copyright (c) 2024 Phuoc Phi
// SPDX-License-Identifier: MPL-2.0
package main

import (
	"github.com/mbver/cserf/cmd/utils"
	"github.com/spf13/cobra"
)

func ActionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "action <name> <payload>",
		Short: "dispatch a cluster action",
		Run: func(cmd *cobra.Command, args []string) {
			out := utils.CreateOutputFromCmd(cmd)
			if len(args) < 1 {
				out.Error(ErrAtLeastOneArg)
				return
			}
			name := args[0]
			payload := []byte{}
			if len(args[0]) > 1 {
				payload = []byte(args[1])
			}

			gClient, err := getClientFromCmd(cmd)
			if err != nil {
				out.Error(err)
				return
			}
			out.Info("connect successfully to server...")
			defer func() {
				gClient.Close()
				out.Info("client closed")
			}()

			_, err = gClient.Action(name, payload)
			if err != nil {
				out.Error(err)
				return
			}
			out.Result("success", nil)
		},
	}
	cmd.Flags().String(FlagRpcAddr, "0.0.0.0:50051", "address of grpc server to connect")
	cmd.Flags().String(FlagCertPath, "./cert", "path to x059 certificate file")
	return cmd
}
