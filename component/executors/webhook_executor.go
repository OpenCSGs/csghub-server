package executors

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"

	"opencsg.com/csghub-server/common/types"
)

type WebHookExecutor interface {
	ProcessEvent(ctx context.Context, event *types.WebHookRecvEvent) error
}

var EventExecutors map[types.WebHookEventType]WebHookExecutor = make(map[types.WebHookEventType]WebHookExecutor)

func RegisterWebHookExecutor(eventType types.WebHookEventType, newExecutor WebHookExecutor) error {
	executor, ok := EventExecutors[eventType]
	if ok {
		executorType := reflect.TypeOf(executor).String()
		return fmt.Errorf("executor %s already registered for webhook event type %s", executorType, eventType)
	}

	newExecutorType := reflect.TypeOf(newExecutor).String()
	EventExecutors[eventType] = newExecutor

	slog.Info("webhook executor registered", slog.Any("event_type", eventType), slog.Any("executor", newExecutorType))

	return nil
}
