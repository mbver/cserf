package main

import (
	"fmt"
	"strings"

	"github.com/mbver/cserf/cmd/utils"
	"github.com/mbver/cserf/rpc/pb"
	"github.com/spf13/cobra"
)

const (
	tagUpdateCommand = "update"
	tagUnsetCommand  = "unset"
)

func isValidTagCommand(s string) bool {
	return s == tagUnsetCommand || s == tagUpdateCommand
}
func TagsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tags <command> <data>",
		Short: "modify tags of the server's serf",
		Long:  tagsText,
		Run: func(cmd *cobra.Command, args []string) {
			out := utils.CreateOutputFromCmd(cmd)
			if len(args) < 2 {
				out.Error(fmt.Errorf("require at least 2 arguments"))
				return
			}
			command := args[0]
			if !isValidTagCommand(command) {
				out.Error(fmt.Errorf("not a valid tag command %s", command))
				return
			}
			if len(args[1]) == 0 {
				out.Error(fmt.Errorf("data should not be empty"))
				return
			}
			req := &pb.TagRequest{}
			req.Command = command
			if command == tagUpdateCommand {
				req.Tags = utils.ToTagMap(args[1])
			}
			if command == tagUnsetCommand {
				req.Keys = strings.Split(args[1], ",")
			}

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

			_, err = gClient.Tag(req)
			if err != nil {
				out.Error(err)
				return
			}
			out.Result(fmt.Sprintf("succeesfully %s tags with %s", command, args[1]), nil)
		},
	}
	cmd.Flags().String(FlagRpcAddr, "0.0.0.0:50051", "address of grpc server to connect")
	cmd.Flags().String(FlagCertPath, "./cert", "path to x059 certificate file")
	return cmd
}

const tagsText = `
modify tags of server's serf and propagate change to all nodes in the cluster

availabe commands are "update" and "unset"

for "update", data in the form "k1=v1,k2=v2"
for "unset", data is in the form "k1,k2"
`
