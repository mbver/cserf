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
		Use:   "keys <command> <key>",
		Short: "manage encryption keys in serf",
		Long:  keyHelp,
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

const keyHelp string = `
Usage: serf keys <command> <key>

  Manage the internal encryption keyring used by Serf. 
  
  Modifications made by this command will be BROADCASTED to all 
  members in the cluster and applied locally on each member.
  
  Operations of this command are IDEMPOTENT.

  To facilitate key rotation, Serf allows for multiple encryption keys to be in
  use simultaneously. Only one key, the "primary" key, will be used for
  encrypting messages. All other keys are used for decryption only.

  Valid commands are: "install", "use", "remove" and "list". "list" do not
  require a key.
  
  WARNING: Running with multiple encryption keys enabled is recommended as a
  TRANSITION state only. Performance may be impacted by using multiple keys.
`
