package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"

	"opencsg.com/csghub-server/tests/testinfra"
)

func main() {
	ctx := context.Background()
	env, err := testinfra.StartTestEnv()
	if err != nil {
		err = errors.Join(err, env.Shutdown(ctx))
		panic(err)
	}
	token, err := env.CreateAdminUser(ctx)
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
	client := testinfra.GetClient(token)
	resp, err := client.Get("http://localhost:9091/api/v1/models?page=1&per=16&search=&sort=trending")
	if err == nil {
		fmt.Println("get models", resp.StatusCode)
	}
	defer resp.Body.Close()

	url := "http://localhost:9091/api/v1/models"
	data := `{"name":"test","nickname":"","namespace":"admin","license":"apache-2.0","description":"","private":false}`
	req, err := http.NewRequest("POST", url, bytes.NewBuffer([]byte(data)))
	if err != nil {
		fmt.Println(err)
		return
	}
	defer resp.Body.Close()
	req.Header.Set("Content-Type", "application/json")
	resp, err = client.Do(req)
	if err != nil {
		fmt.Println(err)
		return
	} else {
		defer resp.Body.Close()
		fmt.Println("create model", resp.StatusCode)
	}

	// Wait for interrupt signal to gracefully shutdown the server with
	// a timeout of 5 seconds.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
	log.Println("Shutdown Server ...")
}
