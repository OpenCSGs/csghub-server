package trigger

import (
	"context"
	"errors"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/user/component"
)

var fixUserDataCmd = &cobra.Command{
	Use:   "fix-user-data",
	Short: "scan user and fix user data",
	RunE: func(cmd *cobra.Command, args []string) error {
		lh := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			AddSource: false,
			Level:     slog.LevelInfo,
		})
		l := slog.New(lh)
		slog.SetDefault(l)

		var users []database.User
		var err error
		var cfg *config.Config
		cfg, err = config.LoadConfig()
		if err != nil {
			return err
		}
		ctx := context.Background()
		userStore := database.NewUserStore()
		userComponent, _ := component.NewUserComponent(cfg)

		// get all organizations
		users, err = userStore.Index(ctx)
		for _, user := range users {
			err1 := userComponent.FixUserData(ctx, user.Username)
			if err1 != nil {
				err = errors.Join(err, err1)
				slog.Error("create user's orgs has error", slog.String("error", err.Error()))
			}
			slog.Info("done create user's orgs", slog.String("org", user.Username))
		}
		return err
	},
}
