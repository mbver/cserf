// Copyright (c) HashiCorp, Inc.
// Copyright (c) 2024 Phuoc Phi
// SPDX-License-Identifier: MPL-2.0
package main

import (
	"fmt"
	"os"

	"github.com/mbver/cserf/rpc/client"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	FlagRpcAddr  = "rpc"
	FlagCertPath = "cert"
)

func getClientFromCmd(cmd *cobra.Command) (*client.Client, error) {
	vp := viper.New()
	vp.BindPFlags(cmd.Flags())
	addr := vp.GetString(FlagRpcAddr)
	cert := vp.GetString(FlagCertPath)
	authKey := os.Getenv("SERF_RPC_AUTH")
	if authKey == "" {
		return nil, fmt.Errorf("empty auth key")
	}
	return client.CreateClient(addr, cert, authKey)
}

func main() {
	cmd := cobra.Command{
		Use: "serf",
	}
	cmd.AddCommand(KeyCommand())
	cmd.AddCommand(ActionCommand())
	cmd.AddCommand(QueryCommand())
	cmd.AddCommand(ReachCommand())
	cmd.AddCommand(MembersCommand())
	cmd.AddCommand(JoinCommand())
	cmd.AddCommand(LeaveCommand())
	cmd.AddCommand(RttCommand())
	cmd.AddCommand(TagsCommand())
	cmd.AddCommand(InfoCommand())
	cmd.AddCommand(KeyGenCommand())
	cmd.AddCommand(MonitorCommand())
	cmd.AddCommand(ConfigCommand())
	cmd.AddCommand(CertGenCommand())
	cmd.AddCommand(ServerCommand())

	cmd.Execute()
}
