package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func isValidKeyCommand(s string) bool {
	return s == "install" || s == "use" || s == "remove" || s == "list"
}

func KeyCommand() *cobra.Command {
	return &cobra.Command{
		Use: "keys <command> <key>",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) < 1 {
				return
			}
			command := args[0]
			if !isValidKeyCommand(command) {
				out.Error(fmt.Errorf("not a key command %q", command))
				return
			}
			key := ""
			if command != "list" {
				if len(args) < 2 {
					// logger.Printf("expect key for command: %q", command)
					return
				}
				key = args[1]
			}
			resp, err := gClient.Key(command, key)
			if err != nil {
				out.Error(err)
				return
			}
			out.Result("key response", resp)
		},
	}
}
