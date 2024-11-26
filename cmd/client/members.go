package main

import (
	"github.com/mbver/cserf/cmd/utils"
	"github.com/spf13/cobra"
)

func MembersCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "members",
		Short: "get the list of all nodes in the cluster",
		Run: func(cmd *cobra.Command, args []string) {
			out := utils.CreateOutputFromCmd(cmd)

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

			res, err := gClient.Members()
			if err != nil {
				out.Error(err)
				return
			}
			out.Result("members", res.Members)
		},
	}
	cmd.Flags().String(FlagRpcAddr, "0.0.0.0:50051", "address of grpc server to connect")
	cmd.Flags().String(FlagCertPath, "./cert", "path to x059 certificate file")
	return cmd
}
