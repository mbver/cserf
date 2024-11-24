package main

import (
	"github.com/mbver/cserf/rpc/pb"
	"github.com/spf13/cobra"
)

func InfoCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "node information and stats",
		Run: func(cmd *cobra.Command, args []string) {
			if !isSetupDone() {
				return
			}
			info, _ := gClient.Info(&pb.Empty{})
			out.Result("info", info)
		},
	}
}
