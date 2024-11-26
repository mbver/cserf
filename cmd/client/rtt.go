package main

import (
	"github.com/mbver/cserf/cmd/utils"
	"github.com/mbver/cserf/rpc/pb"
	"github.com/spf13/cobra"
)

func RttCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rtt <node1> <node2>",
		Short: "get the rtt between two nodes. if node2 is skipped, use the server's serf",
		Run: func(cmd *cobra.Command, args []string) {
			out := utils.CreateOutputFromCmd(cmd)
			if len(args) < 1 {
				out.Error(ErrAtLeastOneArg)
				return
			}
			if len(args) == 1 {
				args = append(args, "") // empty means using the server's node
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

			rtt, err := gClient.Rtt(&pb.RttRequest{
				First:  args[0],
				Second: args[1],
			})
			if err != nil {
				out.Error(err)
				return
			}
			out.Result("rtt", rtt.AsDuration().String())
		},
	}
	cmd.Flags().String(FlagRpcAddr, "0.0.0.0:50051", "address of grpc server to connect")
	cmd.Flags().String(FlagCertPath, "./cert", "path to x059 certificate file")
	return cmd
}
