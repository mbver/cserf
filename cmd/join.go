package main

import (
	"fmt"

	"github.com/mbver/cserf/cmd/utils"
	"github.com/mbver/cserf/rpc/pb"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	FlagIngoreOld = "ignore-old"
)

func JoinCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "join <addr1> <addr2> <addrN>",
		Short: "join existing nodes in cluster",
		Run: func(cmd *cobra.Command, args []string) {
			out := utils.CreateOutputFromCmd(cmd)
			if len(args) < 1 {
				out.Error(ErrAtLeastOneArg)
				return
			}
			vp := viper.New()
			vp.BindPFlags(cmd.Flags())
			ignoreOld := vp.GetBool(FlagIngoreOld)

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
			addrs := args[:]
			res, err := gClient.Join(&pb.JoinRequest{
				Addrs:     addrs,
				IgnoreOld: ignoreOld,
			})
			if err != nil {
				out.Error(err)
				if res == nil {
					return
				}
			}
			out.Result("num of successful joins", fmt.Sprintf("%d/%d", res.Value, len(addrs)))

		},
	}
	cmd.Flags().String(FlagRpcAddr, "0.0.0.0:50051", "address of grpc server to connect")
	cmd.Flags().String(FlagCertPath, "./cert", "path to x059 certificate file")
	cmd.Flags().Bool(FlagIngoreOld, false, "ignore old events from existing nodes while joining")
	return cmd
}
