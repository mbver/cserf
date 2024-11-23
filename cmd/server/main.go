package main

import (
	"github.com/mbver/cserf/cmd/utils"
	"github.com/mbver/cserf/testutils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const FlagRpcAddr = "rpc-addr"

var out = utils.DefaultOutput()

func main() {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "start a grpc server",
		Run: func(cmd *cobra.Command, args []string) {
			vp := viper.New()
			vp.BindPFlags(cmd.Flags())
			addr := vp.GetString(FlagRpcAddr)
			cleanup, err := testutils.CreateTestServer(nil, addr)
			defer cleanup()
			if err != nil {
				out.Error(err)
				return
			}
			out.Infof("running server at %s", addr)
			utils.WaitForTerm(nil)
		},
	}
	cmd.Flags().StringP(FlagRpcAddr, "r", "0.0.0.0:50051", "address that grpc-server listens on")
	cmd.Execute()
}
