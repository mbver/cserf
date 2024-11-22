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
			out.Result("num of successful joins", fmt.Sprintf("%d/%d", res.Value, len(addrs)))
			if err != nil {
				out.Error(err)
			}
		},
	}
	cmd.Flags().String(FlagAddrs, "", `list of existing nodes' addresses to join, separated by ","`)
	cmd.Flags().Bool(FlagIngoreOld, false, "ignore old events from existing nodes while joining")
	return cmd
}
