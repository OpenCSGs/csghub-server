package main

import (
	"context"
	"os"

	"opencsg.com/starhub-server/cmd/starhub-server/cmd"
	"opencsg.com/starhub-server/common/log"
	"opencsg.com/starhub-server/common/log/trace"
)

func main() {
	closer := trace.InitTracing()
	defer closer.Close()

	defer log.Sync()

	command := cmd.RootCmd
	if err := command.ExecuteContext(context.Background()); err != nil {
		os.Exit(1)
	}
}
