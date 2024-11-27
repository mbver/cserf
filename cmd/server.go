package main

import (
	"github.com/mbver/cserf/cmd/utils"
	"github.com/mbver/cserf/rpc/server"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	FlagConfig = "conf"
)

func ServerCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "start a grpc server",
		Run: func(cmd *cobra.Command, args []string) {
			out := utils.CreateOutputFromCmd(cmd)
			vp := viper.New()
			vp.BindPFlags(cmd.Flags())
			confPath := vp.GetString(FlagConfig)
			conf, err := server.LoadConfig(confPath)
			if err != nil {
				out.Error(err)
				return
			}

			_, cleanup1, err := server.CreateServer(conf)
			defer cleanup1()
			if err != nil {
				out.Error(err)
				return
			}
			out.Infof("running server at %s:%d", conf.RpcAddress, conf.RpcPort)
			utils.WaitForTerm(nil)
		},
	}
	cmd.Flags().String(FlagConfig, "./config.yaml", "path to YAML server-config file")
	return cmd
}
