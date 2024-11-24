package main

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func ReachCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "reach",
		Short: "test reachability of nodes in the cluster",
		Run: func(cmd *cobra.Command, args []string) {
			if !isSetupDone() {
				return
			}
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
			out.Result("response counts:", fmt.Sprintf("%d/%d", res.NumRes, res.NumNode))
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
}
