package main

import "github.com/spf13/cobra"

func ActiveCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "active",
		Short: "get the list of active nodes in the cluster",
		Run: func(cmd *cobra.Command, args []string) {
			if !isSetupDone() {
				return
			}
			res, err := gClient.Active()
			if err != nil {
				out.Error(err)
				return
			}
			out.Result("active-members", res.Members)
		},
	}
}
