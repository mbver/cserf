// Copyright (c) HashiCorp, Inc.
// Copyright (c) 2024 Phuoc Phi
// SPDX-License-Identifier: MPL-2.0
package main

import (
	"github.com/mbver/cserf/cmd/utils"
	"github.com/mbver/cserf/rpc/pb"
	"github.com/mbver/cserf/rpc/server"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	FlagEventFilter = "filter"
	FlagLogLevel    = "level"
)

func MonitorCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "monitor",
		Short: "monitor events and logs from the server's serf",
		Run: func(cmd *cobra.Command, args []string) {
			out := utils.CreateOutputFromCmd(cmd)
			vp := viper.New()
			vp.BindPFlags(cmd.Flags())
			filter := vp.GetString(FlagEventFilter)
			level := vp.GetString(FlagLogLevel)

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

			stream, cancel, err := gClient.Monitor(&pb.MonitorRequest{
				EventFilter: filter,
				LogLevel:    level,
			})
			defer cancel()
			if err != nil {
				out.Error(err)
				return
			}
			out.Result("streaming now", nil)
			term := make(chan struct{})
			done := make(chan struct{})
			stopWait := make(chan struct{})
			outCh := make(chan string, 1024)
			errCh := make(chan error, 1)
			go func() {
				for {
					line, err := stream.Recv()
					if err != nil {
						errCh <- err
						if server.ShouldStopStreaming(err) {
							close(stopWait)
							return
						}
						continue
					}
					outCh <- line.Value
				}
			}()
			go func() {
				for {
					select {
					case <-term:
						close(done)
						return
					case s := <-outCh:
						out.Info(s)
					case err := <-errCh:
						out.Error(err)
					}
				}
			}()
			utils.WaitForTerm(stopWait)
			close(term)
			<-done
			out.Result("streaming terminated", nil)
		},
	}
	cmd.Flags().String(FlagRpcAddr, "0.0.0.0:50051", "address of grpc server to connect")
	cmd.Flags().String(FlagCertPath, "./cert", "path to x059 certificate file")
	cmd.Flags().String(FlagEventFilter, "*", "the type of events to be moninored")
	cmd.Flags().String(FlagLogLevel, "INFO", "the minimum log level to stream")
	return cmd
}
