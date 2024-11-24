package main

import (
	"fmt"
	"strings"

	"github.com/mbver/cserf/rpc/pb"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	FlagAddrs     = "addrs"
	FlagIngoreOld = "ignore-old"
)

func JoinCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "join",
		Short: "join existing nodes in cluster",
		Run: func(cmd *cobra.Command, args []string) {
			if !isSetupDone() {
				return
			}
			vp := viper.New()
			vp.BindPFlags(cmd.Flags())
			addrStr := vp.GetString(FlagAddrs)
			if len(addrStr) == 0 {
				out.Error(fmt.Errorf("no node to join"))
				return
			}
			addrs := strings.Split(addrStr, ",")
			ignoreOld := vp.GetBool(FlagIngoreOld)
			res, err := gClient.Join(&pb.JoinRequest{
				Addrs:     addrs,
				IgnoreOld: ignoreOld,
			})
			if err != nil {
				out.Error(err)
				if res == nil {
					return
				}
			}
			out.Result("num of successful joins", fmt.Sprintf("%d/%d", res.Value, len(addrs)))

		},
	}
	cmd.Flags().String(FlagAddrs, "", `list of existing nodes' addresses to join, separated by ","`)
	cmd.Flags().Bool(FlagIngoreOld, false, "ignore old events from existing nodes while joining")
	return cmd
}
