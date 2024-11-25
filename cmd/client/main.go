package main

import (
	"fmt"
	"os"

	"github.com/mbver/cserf/cmd/utils"
	"github.com/mbver/cserf/rpc/client"
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
			name := cmd.Name()
			if name == "keygen" || name == "config" || name == "cert" {
				return
			}
			vp := viper.New()
			vp.BindPFlags(cmd.Flags())
			addr := vp.GetString(FlagRpcAddr)
			cert := vp.GetString(FlagCertPath)
			authKey := os.Getenv("SERF_RPC_AUTH")
			if authKey == "" {
				out.Error(fmt.Errorf("no auth key"))
				os.Exit(1)
			}
			var err error
			gClient, err = client.CreateClient(addr, cert, authKey)
			if err != nil {
				out.Error(err)
				os.Exit(1)
			}
			out.Infof("connected to server at %s", addr)
		},
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			if gClient != nil {
				out.Info("closing client ...")
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
	cmd.AddCommand(LeaveCommand())
	cmd.AddCommand(RttCommand())
	cmd.AddCommand(TagsCommand())
	cmd.AddCommand(InfoCommand())
	cmd.AddCommand(KeyGenCommand())
	cmd.AddCommand(MonitorCommand())
	cmd.AddCommand(ConfigCommand())
	cmd.AddCommand(CertGenCommand())

	cmd.Execute()
}
