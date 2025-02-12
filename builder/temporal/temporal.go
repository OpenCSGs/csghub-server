package temporal

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/contrib/opentelemetry"
	"go.temporal.io/sdk/interceptor"
	"go.temporal.io/sdk/worker"
)

type Client interface {
	client.Client
	NewWorker(queue string, options worker.Options) worker.Registry
	GetScheduleClient() ScheduleClient
	Start() error
	Stop()
}

type ScheduleClient interface {
	Create(ctx context.Context, options client.ScheduleOptions) (client.ScheduleHandle, error)
}

type clientImpl struct {
	client.Client
	workers []worker.Worker
}

var _client Client = &clientImpl{}

func NewClient(options client.Options, serviceName string) (*clientImpl, error) {
	tracingInterceptor, err := opentelemetry.NewTracingInterceptor(opentelemetry.TracerOptions{
		Tracer: otel.Tracer(serviceName),
	})
	if err != nil {
		return nil, fmt.Errorf("temporal otel interceptor %w", err)
	}
	options.Interceptors = []interceptor.ClientInterceptor{tracingInterceptor}

	t, err := client.Dial(options)
	if err != nil {
		return nil, err
	}
	c := _client.(*clientImpl)
	c.Client = t

	return c, nil
}

// used in test only
func Assign(temporalClient client.Client) {
	c := _client.(*clientImpl)
	c.Client = temporalClient

}

func (c *clientImpl) NewWorker(queue string, options worker.Options) worker.Registry {
	w := worker.New(c.Client, queue, options)
	c.workers = append(c.workers, w)
	return w
}

func (c *clientImpl) GetScheduleClient() ScheduleClient {
	return c.ScheduleClient()
}

func (c *clientImpl) Start() error {
	for _, worker := range c.workers {
		err := worker.Start()
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *clientImpl) Stop() {
	for _, worker := range c.workers {
		worker.Stop()
	}

	if c.Client != nil {
		c.Close()
	}
}

func GetClient() Client {
	return _client
}

func Stop() {
	_client.Close()
}
