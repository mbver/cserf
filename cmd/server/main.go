package main

import (
	"github.com/mbver/cserf/cmd/utils"
	"github.com/mbver/cserf/rpc/server"
	"github.com/mbver/mlist/testaddr"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const FlagRpcAddr = "rpc-addr"

var out = utils.DefaultOutput()

// TODO: read config from files or flag. don't use testutils anymore!
func main() {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "start a grpc server",
		Run: func(cmd *cobra.Command, args []string) {
			conf, cleanup, err := defaultServerConfig()
			out.Info(conf.CertPath)
			defer cleanup()
			if err != nil {
				out.Error(err)
				return
			}
			vp := viper.New()
			vp.BindPFlags(cmd.Flags())
			addr := vp.GetString(FlagRpcAddr) // NOT USE NOW
			cleanup1, err := server.CreateServer(conf)
			defer cleanup1()
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

func defaultServerConfig() (*server.ServerConfig, func(), error) {
	ip, cleanup := testaddr.BindAddrs.NextAvailAddr()
	certPath, keyPath, cleanup1, err := server.GenerateSelfSignedCert()
	cleanup2 := server.CombineCleanup(cleanup1, cleanup)
	if err != nil {
		return nil, cleanup2, err
	}
	return &server.ServerConfig{
		RpcAddress: "127.0.0.1",
		RpcPort:    50051,
		BindAddr:   ip.String(),
		BindPort:   7946,
		CertPath:   certPath,
		KeyPath:    keyPath,
	}, cleanup2, nil
}
