package mirror

import (
	"fmt"

	"github.com/spf13/cobra"
	"opencsg.com/csghub-server/api/workflow"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/mirror"
)

var repoSyncCmd = &cobra.Command{
	Use:     "repo-sync",
	Short:   "Start the repoisotry sync server",
	Example: repoSyncExample(),
	RunE: func(*cobra.Command, []string) (err error) {
		cfg, err := config.LoadConfig()
		if err != nil {
			return err
		}

		dbConfig := database.DBConfig{
			Dialect: database.DatabaseDialect(cfg.Database.Driver),
			DSN:     cfg.Database.DSN,
		}
		database.InitDB(dbConfig)

		repoSYncer, err := mirror.NewRepoSyncWorker(cfg, cfg.Mirror.WorkerNumber)
		if err != nil {
			return err
		}
		err = workflow.StartWorker(cfg)
		if err != nil {
			return fmt.Errorf("failed to start worker:  %w", err)
		}

		repoSYncer.Run()
		workflow.StopWorker()

		return nil
	},
}

func repoSyncExample() string {
	return `
# for development
csghub-server mirror repo-sync
`
}
