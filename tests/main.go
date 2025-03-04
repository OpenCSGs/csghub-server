package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"opencsg.com/csghub-server/tests/testinfra"
)

func main() {
	ctx := context.Background()
	env, err := testinfra.StartTestEnv()
	defer func() { _ = env.Shutdown(ctx) }()
	if err != nil {
		panic(err)
	}
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
	log.Println("Shutdown Server ...")
}
