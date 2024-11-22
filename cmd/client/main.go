package main

import (
	"github.com/mbver/cserf/cmd/utils"
	"github.com/mbver/cserf/rpc/client"
	"github.com/mbver/cserf/testutils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	FlagRpcAddr  = "rpc"
	FlagCertPath = "cert"
)

var out = utils.DefaultOutput()
var gClient *client.Client

func main() {
	cmd := cobra.Command{
		Use: "serf",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			vp := viper.New()
			vp.BindPFlags(cmd.Flags())
			addr := vp.GetString(FlagRpcAddr)
			cert := vp.GetString(FlagCertPath)
			var err error
			gClient, err = testutils.CreateTestClient(addr, cert)
			out.Infof("connected to server at %s", addr)
			if err != nil {
				out.Error(err)
				return
			}
		},
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			out.Info("closing client ...")
			if gClient != nil {
				gClient.Close()
			}
		},
	}
	cmd.PersistentFlags().String(FlagRpcAddr, "0.0.0.0:50051", "address of grpc server to connect")
	cmd.PersistentFlags().String(FlagCertPath, "./cert", "path to x059 certificate file")

	cmd.AddCommand(KeyCommand())
	cmd.AddCommand(ActionCommand())
	cmd.AddCommand(QueryCommand())
	cmd.AddCommand(ActiveCommand())
	cmd.AddCommand(ReachCommand())
	cmd.AddCommand(MembersCommand())
	cmd.AddCommand(JoinCommand())

	cmd.Execute()
}
