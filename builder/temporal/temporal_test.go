package temporal_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
	"go.temporal.io/server/temporaltest"
	"opencsg.com/csghub-server/builder/temporal"
)

type Tester struct {
	client  temporal.Client
	counter int
	total   int
}

func (t *Tester) Count(ctx workflow.Context) error {
	t.counter += 1
	return nil
}

func (t *Tester) Add(ctx workflow.Context) error {
	t.total += 1
	return nil
}

func TestTemporalClient(t *testing.T) {
	ts := temporaltest.NewServer(temporaltest.WithT(t))
	defer ts.Stop()
	c := ts.GetDefaultClient()
	temporal.Assign(c)

	tc, _ := temporal.NewClient(client.Options{}, "test")
	tester := &Tester{client: tc}

	worker1 := tester.client.NewWorker("q1", worker.Options{})
	worker1.RegisterWorkflow(tester.Count)
	worker2 := tester.client.NewWorker("q2", worker.Options{})
	worker2.RegisterWorkflow(tester.Add)

	err := tester.client.Start()
	require.NoError(t, err)

	r, err := tester.client.ExecuteWorkflow(context.TODO(), client.StartWorkflowOptions{
		TaskQueue: "q1",
	}, tester.Count)
	require.NoError(t, err)
	err = r.Get(context.Background(), nil)
	require.NoError(t, err)

	r, err = tester.client.ExecuteWorkflow(context.TODO(), client.StartWorkflowOptions{
		TaskQueue: "q2",
	}, tester.Add)
	require.NoError(t, err)
	err = r.Get(context.Background(), nil)
	require.NoError(t, err)

	require.Equal(t, 1, tester.counter)
	require.Equal(t, 1, tester.total)

	temporal.Stop()
	_, err = tester.client.ExecuteWorkflow(context.TODO(), client.StartWorkflowOptions{
		TaskQueue: "q1",
	}, tester.Count)
	require.Error(t, err)

}
