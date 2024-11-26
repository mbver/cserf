package main

import (
	"fmt"
	"time"

	"github.com/mbver/cserf/cmd/utils"
	"github.com/spf13/cobra"
)

func ReachCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reach",
		Short: "test reachability of nodes in the cluster",
		Run: func(cmd *cobra.Command, args []string) {
			out := utils.CreateOutputFromCmd(cmd)

			gClient, err := getClientFromCmd(cmd)
			if err != nil {
				out.Error(err)
				return
			}
			out.Info("connect successfully to server...")
			defer func() {
				gClient.Close()
				out.Info("client closed")
			}()

			actives, err := gClient.Active()
			if err != nil {
				out.Error(err)
			}
			out.Info("running reachability test ...")
			start := time.Now()
			res, err := gClient.Reach()
			out.Result("took", time.Since(start).String())
			if err != nil {
				out.Error(err)
				return
			}
			out.Result("response counts", fmt.Sprintf("%d/%d", res.NumRes, res.NumNode))
			if res.NumRes < res.NumNode {
				out.Error(fmt.Errorf("too few responses: %d/%d", res.NumRes, res.NumNode))
			}
			m := make(map[string]struct{})
			for _, id := range res.Acked {
				m[id] = struct{}{}
			}
			for _, n := range actives.Members {
				if _, ok := m[n.Id]; !ok {
					out.Error(fmt.Errorf("missing: %s, %s", n.Id, n.Addr))
				}
			}
		},
	}
	cmd.Flags().String(FlagRpcAddr, "0.0.0.0:50051", "address of grpc server to connect")
	cmd.Flags().String(FlagCertPath, "./cert", "path to x059 certificate file")
	return cmd
}
