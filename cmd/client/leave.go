package main

import (
	"github.com/mbver/cserf/rpc/pb"
	"github.com/spf13/cobra"
)

func LeaveCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "leave",
		Short: "leave the cluster",
		Run: func(cmd *cobra.Command, args []string) {
			if !isSetupDone() {
				return
			}
			out.Info("gonna leave, don't be clingy to me ...")
			_, err := gClient.Leave(&pb.Empty{})
			if err != nil {
				out.Error(err)
				return
			}
			out.Result("leave successfully", nil)
		},
	}
}
