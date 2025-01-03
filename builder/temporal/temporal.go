package temporal

import (
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

type Client interface {
	client.Client
	NewWorker(queue string, options worker.Options) worker.Registry
	Start() error
	Stop()
}

type clientImpl struct {
	client.Client
	workers []worker.Worker
}

var _client Client = &clientImpl{}

func NewClient(temporalClient client.Client) (*clientImpl, error) {
	c := _client.(*clientImpl)
	c.Client = temporalClient

	return c, nil
}

func (c *clientImpl) NewWorker(queue string, options worker.Options) worker.Registry {
	w := worker.New(c.Client, queue, options)
	c.workers = append(c.workers, w)
	return w
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
