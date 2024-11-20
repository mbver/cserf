package main

import (
	"log"
	"os"

	"github.com/spf13/cobra"
)

var logger = log.New(os.Stderr, "serf-client: ", log.LstdFlags)

func main() {
	rootCmd := cobra.Command{
		Use: "serf",
	}
	rootCmd.AddCommand(KeyCommand())
	rootCmd.Execute()
}
