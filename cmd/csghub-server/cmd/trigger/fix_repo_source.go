package trigger

import (
	"context"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/component"
)

var fixRepoSourceCmd = &cobra.Command{
	Use:   "fix-repo-source",
	Short: "Generate ms_path hf_path csgpath for all mirror repositories",
	RunE: func(cmd *cobra.Command, args []string) error {
		lh := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			AddSource: false,
			Level:     slog.LevelInfo,
		})
		l := slog.New(lh)
		slog.SetDefault(l)

		var err error
		var cfg *config.Config
		cfg, err = config.LoadConfig()
		if err != nil {
			return err
		}
		ctx := context.Background()
		repoComponent, _ := component.NewRepoComponent(cfg)
		err = repoComponent.FixRepoSource(ctx)
		if err != nil {
			return err
		}
		return nil
	},
}
