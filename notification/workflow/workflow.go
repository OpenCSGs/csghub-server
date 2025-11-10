package workflow

import (
	"opencsg.com/csghub-server/builder/temporal"
	"opencsg.com/csghub-server/common/config"
)

func RegisterWorker(cfg *config.Config, wfClient temporal.Client) {
	// create worker for each channel
	createWorker(cfg, wfClient)
}
