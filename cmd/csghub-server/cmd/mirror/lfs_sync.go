package mirror

import (
	"github.com/spf13/cobra"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/mirror"
)

var lfsSyncCmd = &cobra.Command{
	Use:     "lfs-sync",
	Short:   "Start the repoisotry lfs files sync server",
	Example: lfsSyncExample(),
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

		lfsSyncWorker, err := mirror.NewLFSSyncWorker(cfg, cfg.Mirror.WorkerNumber)
		if err != nil {
			return err
		}
		lfsSyncWorker.Run()

		return nil
	},
}

func lfsSyncExample() string {
	return `
# for development
csghub-server mirror lfs-sync
`
}
