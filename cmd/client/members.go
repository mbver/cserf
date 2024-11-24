package main

import "github.com/spf13/cobra"

func MembersCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "members",
		Short: "get the list of all nodes in the cluster",
		Run: func(cmd *cobra.Command, args []string) {
			if !isSetupDone() {
				return
			}
			res, err := gClient.Members()
			if err != nil {
				out.Error(err)
				return
			}
			out.Result("members", res.Members)
		},
	}
}
