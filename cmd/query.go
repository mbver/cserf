// Copyright (c) HashiCorp, Inc.
// Copyright (c) 2024 Phuoc Phi
// SPDX-License-Identifier: MPL-2.0
package main

import (
	"fmt"
	"time"

	"github.com/mbver/cserf/cmd/utils"
	"github.com/mbver/cserf/rpc/pb"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"google.golang.org/protobuf/types/known/durationpb"
)

const (
	FlagName       = "name"
	FlagNodeFilter = "nodes"
	FlagTag        = "tags"
	FlagTimeout    = "timeout"
	FlagRelay      = "relay"
	FlagPayload    = "payload"
)

func QueryCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "query",
		Short: "run a query over all nodes in the cluster",
		Run: func(cmd *cobra.Command, args []string) {
			out := utils.CreateOutputFromCmd(cmd)
			vp := viper.New()
			vp.BindPFlags(cmd.Flags())
			name := vp.GetString(FlagName)
			if name == "" {
				out.Error(fmt.Errorf("query name is required"))
				return
			}
			nodeStr := vp.GetString(FlagNodeFilter)
			tagStr := vp.GetString(FlagTag)
			timeout := vp.GetDuration(FlagTimeout)
			relay := vp.GetInt(FlagRelay)
			if relay > 255 {
				out.Error(fmt.Errorf("relay factor exceeds limit: %d/%d", relay, 255))
				return
			}
			payload := vp.GetString(FlagPayload)
			p := toQueryParams(name, nodeStr, tagStr, timeout, relay, payload)

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

			resCh, cancel, err := gClient.Query(p)
			defer cancel()

			if err != nil {
				out.Error(err)
				return
			}
			for r := range resCh {
				out.Info(r)
			}
		},
	}
	cmd.Flags().String(FlagRpcAddr, "0.0.0.0:50051", "address of grpc server to connect")
	cmd.Flags().String(FlagCertPath, "./cert", "path to x059 certificate file")
	cmd.Flags().String(FlagName, "", "query name")
	cmd.Flags().String(FlagNodeFilter, "", "query targets, empty means all nodes")
	cmd.Flags().String(FlagTag, "", "tags filter. empty means no filter")
	cmd.Flags().Duration(FlagTimeout, 0, "query timeout parameter")
	cmd.Flags().Int(FlagRelay, 0, "relay factor")
	cmd.Flags().String(FlagPayload, "", "payload of query")
	return cmd
}

func toQueryParams(
	name string,
	nodeStr string,
	tagStr string,
	timeout time.Duration,
	relay int,
	payload string,
) *pb.QueryParam {

	tags := toFilterTags(tagStr)
	return &pb.QueryParam{
		Name:       name,
		ForNodes:   utils.ToNodes(nodeStr),
		FilterTags: tags,
		NumRelays:  uint32(relay),
		Timeout:    durationpb.New(timeout),
		Payload:    []byte(payload),
	}
}

func toFilterTags(s string) []*pb.FilterTag {
	m := utils.ToTagMap(s)
	if len(m) == 0 {
		return nil
	}
	filters := make([]*pb.FilterTag, 0, len(m))
	for k, v := range m {
		filters = append(filters, &pb.FilterTag{
			Name: k,
			Expr: v,
		})
	}
	return filters
}
