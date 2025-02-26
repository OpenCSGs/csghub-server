package testserver

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"

	"opencsg.com/csghub-server/tests/testinfra"
)

func StartTestServer() {
	ctx := context.Background()
	env, err := testinfra.StartTestEnv()
	if err != nil {
		err = errors.Join(err, env.Shutdown(ctx))
		panic(err)
	}
	defer func() {
		err = env.Shutdown(ctx)
		if err != nil {
			fmt.Println("shutdown test env error: ", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
	log.Println("Shutdown Server ...")
}
