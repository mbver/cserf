package main

import (
	"github.com/mbver/cserf/testutils"
	"github.com/spf13/cobra"
)

func isValidKeyCommand(s string) bool {
	return s == "install" || s == "use" || s == "remove" || s == "list"
}

func KeyCommand() *cobra.Command {
	return &cobra.Command{
		Use: "keys",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) < 1 {
				logger.Println("at least 1 argument required")
			}
			command := args[0]
			if !isValidKeyCommand(command) {
				logger.Printf("not a key command %s", command)
				return
			}
			if command != "list" && len(args) < 2 {
				logger.Printf("key is required for %s", args[0])
				return
			}
			key := ""
			if command != "list" {
				key = args[1]
			}

			client, _, cleanup, err := testutils.ClientServerRPC(nil)
			defer cleanup()
			if err != nil {
				logger.Printf("error: %v", err)
				return
			}

			resp, err := client.Key(command, key)
			if err != nil {
				logger.Printf("error: %v", err)
				return
			}
			logger.Printf("========= response: %+v", resp)
		},
	}
}
