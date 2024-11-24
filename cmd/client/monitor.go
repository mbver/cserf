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
)

func MonitorCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "monitor",
		Short: "monitor events and logs from the server's serf",
		Run: func(cmd *cobra.Command, args []string) {
			if !isSetupDone() {
				return
			}
			vp := viper.New()
			vp.BindPFlags(cmd.Flags())
			filter := vp.GetString(FlagEventFilter)
			stream, cancel, err := gClient.Monitor(&pb.StringValue{
				Value: filter,
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
	cmd.Flags().String(FlagEventFilter, "*", "the type of events to be moninored")
	return cmd
}
