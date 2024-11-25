package main

import (
	"github.com/spf13/cobra"
)

func ActionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "action <name> <payload>",
		Short: "dispatch a cluster action",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) < 1 {
				out.Error(ErrAtLeastOneArg)
				return
			}
			name := args[0]
			payload := []byte{}
			if len(args[0]) > 1 {
				payload = []byte(args[1])
			}
			_, err := gClient.Action(name, payload)
			if err != nil {
				out.Error(err)
				return
			}
			out.Result("success", nil)
		},
	}
}
