package main

import (
	"context"
	"os"

	"git-devops.opencsg.com/product/community/starhub-server/cmd/starhub-server/cmd"
	"git-devops.opencsg.com/product/community/starhub-server/cmd/starhub-server/cmd/start"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/log"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/log/trace"
)

func main() {
	closer := trace.InitTracing()
	defer closer.Close()

	defer log.Sync()

	start.Initializer = initAPIServer
	command := cmd.RootCmd
	if err := command.ExecuteContext(context.Background()); err != nil {
		os.Exit(1)
	}
}
