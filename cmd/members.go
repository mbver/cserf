package main

import (
	"fmt"
	"strings"

	"github.com/mbver/cserf/cmd/utils"
	"github.com/mbver/cserf/rpc/pb"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func MembersCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "members",
		Short: "get the list of all nodes in the cluster",
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

			vp := viper.New()
			vp.BindPFlags(cmd.Flags())
			filterStr := vp.GetString(FlagTag)
			tagFilters, err := toTagFilters(filterStr)
			if err != nil {
				out.Error(err)
				return
			}
			res, err := gClient.Members(&pb.MemberRequest{
				TagFilters: tagFilters,
			})
			if err != nil {
				out.Error(err)
				return
			}
			out.Result("members", res.Members)
		},
	}
	cmd.Flags().String(FlagRpcAddr, "0.0.0.0:50051", "address of grpc server to connect")
	cmd.Flags().String(FlagCertPath, "./cert", "path to x059 certificate file")
	cmd.Flags().String(FlagTag, "", "tags filters, in the form k1=expr1,k2=expr2")
	return cmd
}

func toTagFilters(str string) ([]*pb.TagFilter, error) {
	if len(str) == 0 {
		return []*pb.TagFilter{}, nil
	}
	split := strings.Split(str, ",")
	kvs := make([][2]string, len(split))
	for i, s := range split {
		split1 := strings.Split(s, "=")
		if len(split1) != 2 {
			return nil, fmt.Errorf("invalid tag filter %s", s)

		}
		kvs[i][0], kvs[i][1] = split1[0], split1[1]
	}
	var tagFilters = make([]*pb.TagFilter, len(kvs))
	for i, kv := range kvs {
		tagFilters[i] = &pb.TagFilter{
			Key:  kv[0],
			Expr: kv[1],
		}
	}
	return tagFilters, nil
}
