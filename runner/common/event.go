package common

import (
	"container/list"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/common/types"
)

var (
	failedEventCache = list.New()
	eventCacheLock   sync.Mutex
)

func Push(address, apiKey string, event *types.WebHookSendEvent) error {
	if len(address) < 1 {
		slog.Warn("no webhook address found for status event")
		return nil
	}

	client := rpc.NewHttpClient(address, rpc.AuthWithApiKey(apiKey)).WithRetry(3).WithDelay(time.Second * 2)
	urlPath := "/api/v1/webhook/runner"
	var (
		err        error
		statusCode int
	)
	statusCode, err = pushWithRetry(client, urlPath, event)

	if err != nil {
		failedEventCache.PushBack(*event)
		return fmt.Errorf("failed to push event to %s%s with error: %w", address, urlPath, err)
	}

	if statusCode != http.StatusOK {
		failedEventCache.PushBack(*event)
		return fmt.Errorf("failed to push event to %s%s with response code %d", address, urlPath, statusCode)
	}

	return nil
}

func PushCachedFailedEvents(address, apiKey string) {
	if eventCacheLock.TryLock() {
		defer eventCacheLock.Unlock()

		for failedEventCache.Len() > 0 {
			first := failedEventCache.Front()
			if first == nil {
				return
			}

			event, ok := first.Value.(*types.WebHookSendEvent)
			if !ok {
				failedEventCache.Remove(first)
				continue
			}

			failedEventCache.Remove(first)

			err := Push(address, apiKey, event)
			if err != nil {
				slog.Error("failed to re-send failed event", "error", err)
			}
		}
	}
}

func pushWithRetry(client *rpc.HttpClient, urlPath string, event *types.WebHookSendEvent) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := client.PostResponse(ctx, urlPath, event)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	return resp.StatusCode, nil
}
