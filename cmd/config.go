// Copyright (c) HashiCorp, Inc.
// Copyright (c) 2024 Phuoc Phi
// SPDX-License-Identifier: MPL-2.0
package main

import (
	"os"
	"path/filepath"

	"github.com/mbver/cserf/cmd/utils"
	"github.com/mbver/cserf/rpc/server"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

const FlagTestConfig = "test"

func ConfigCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config <path>",
		Short: "create a YAML config file",
		Run: func(cmd *cobra.Command, args []string) {
			out := utils.CreateOutputFromCmd(cmd)
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

			vp := viper.New()
			vp.BindPFlags(cmd.Flags())
			testConf := vp.GetBool(FlagTestConfig)
			conf := server.DefaultServerConfig()
			if testConf {
				conf, err = utils.CreateTestServerConfig()
				if err != nil {
					out.Error(err)
					return
				}
				dir := filepath.Dir(path)
				scriptname := "eventscript.sh"
				err := utils.CreateTestEventScript(dir, scriptname)
				if err != nil {
					out.Error(err)
					return
				}
				conf.SerfConfig.EventScript = scriptname
				keyfilename := "keyring.json"
				err = utils.CreateTestKeyringFile(dir, keyfilename, []string{"T9jncgl9mbLus+baTTa7q7nPSUrXwbDi2dhbtqir37s="})
				if err != nil {
					out.Error(err)
					return
				}
				conf.SerfConfig.KeyringFile = keyfilename
			}
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
	cmd.Flags().Bool(FlagTestConfig, false, "set it to create a test-server-config instead of a default")
	return cmd
}
