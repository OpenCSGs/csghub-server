package worker

import (
	"log/slog"

	"opencsg.com/csghub-server/builder/temporal"
	"opencsg.com/csghub-server/common/config"
)

// WorkerCreator defines the function signature for creating a Temporal worker.
type WorkerCreator func(cfg *config.Config, client temporal.Client)

var workerCreators = make(map[string]WorkerCreator)

// RegisterWorker is called by channel implementations to register their worker creation function.
func RegisterWorker(name string, creator WorkerCreator) {
	if _, ok := workerCreators[name]; ok {
		slog.Warn("worker creator already registered, it will be overwritten", "name", name)
	}
	workerCreators[name] = creator
}

// GetWorkerCreators returns all registered worker creators.
func GetWorkerCreators() map[string]WorkerCreator {
	return workerCreators
}
