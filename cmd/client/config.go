package main

import (
	"os"

	"github.com/mbver/cserf/rpc/server"
	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"
)

func ConfigCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "config <path>",
		Short: "create a YAML default config file",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) < 1 {
				out.Error(ErrAtLeastOneArg)
				return
			}
			path := args[0]
			fh, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0644)
			if err != nil {
				out.Error(err)
				return
			}
			defer fh.Close()

			conf := server.DefaultServerConfig()
			ybytes, err := yaml.Marshal(conf)
			if err != nil {
				out.Error(err)
				return
			}
			_, err = fh.Write(ybytes)
			if err != nil {
				out.Error(err)
				return
			}
			out.Result("successfully write a default-server-config file to", path)
		},
	}
}
