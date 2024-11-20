package main

import (
	"log"
	"os"

	"github.com/mbver/cserf/testutils"
	"github.com/spf13/cobra"
)

var logger = log.New(os.Stderr, "grpc-server:", log.LstdFlags)

func main() {
	rootCmd := &cobra.Command{
		Use:   "server",
		Short: "start a grpc server",
		Run: func(cmd *cobra.Command, args []string) {
			client, _, cleanup, err := testutils.ClientServerRPC(nil)
			defer cleanup()
			if err != nil {
				logger.Printf("error: %v", err)
				return
			}
			logger.Println("======= started serf")
			res, err := client.Hello("alex")
			if err != nil {
				logger.Printf("error: %v", err)
				return
			}
			logger.Printf("====== resp from server: %v", res)
		},
	}
	rootCmd.Execute()
}
