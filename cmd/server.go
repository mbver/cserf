// Copyright (c) HashiCorp, Inc.
// Copyright (c) 2024 Phuoc Phi
// SPDX-License-Identifier: MPL-2.0
package main

import (
	"fmt"
	"time"

	serf "github.com/mbver/cserf"
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

			s, cleanup1, err := server.CreateServer(conf)

			var rejoinFailCh chan struct{}

			if len(conf.RetryJoins) > 0 {
				rejoinFailCh = make(chan struct{})
				go func() {
					waitTime := conf.RetryJoinInterval*time.Duration(conf.RetryJoinMax) + 10*time.Millisecond
					for i := 0; i < 3; i++ {
						time.Sleep(waitTime)
						if s.State() == serf.SerfShutdown {
							out.Error(fmt.Errorf("failed to rejoin %v, serf is shutdown now", conf.RetryJoins))
							close(rejoinFailCh)
							return
						}
					}
				}()
			}

			defer cleanup1()
			if err != nil {
				out.Error(err)
				return
			}
			out.Infof("running server at %s:%d", conf.RpcAddress, conf.RpcPort)
			utils.WaitForTerm(rejoinFailCh)
			out.Infof("server exitting ...")
		},
	}
	cmd.Flags().String(FlagConfig, "./config.yaml", "path to YAML server-config file")
	return cmd
}
