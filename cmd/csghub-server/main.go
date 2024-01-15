package main

import (
	"context"
	"os"

	"opencsg.com/csghub-server/cmd/csghub-server/cmd"
)

func main() {
	command := cmd.RootCmd
	if err := command.ExecuteContext(context.Background()); err != nil {
		os.Exit(1)
	}
}
