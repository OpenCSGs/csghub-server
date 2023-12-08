package main

import (
	"context"
	"os"

	"opencsg.com/starhub-server/cmd/starhub-server/cmd"
	"opencsg.com/starhub-server/cmd/starhub-server/cmd/start"
	"opencsg.com/starhub-server/pkg/log"
	"opencsg.com/starhub-server/pkg/log/trace"
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
