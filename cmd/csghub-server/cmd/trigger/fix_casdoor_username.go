package trigger

import (
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/casdoor/casdoor-go-sdk/casdoorsdk"
	"github.com/spf13/cobra"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
)

var fixCasdoorUsernameCmd = &cobra.Command{
	Use:   "fix-casdoor-username",
	Short: "fix inconsistent casdoor Name by syncing it with local DB username",
	Long: `Scan all casdoor users in the local database, compare their casdoor Name field
with the local DB username, and update casdoor Name to match if they differ.

This fixes the inconsistency caused by WeChat login users changing their username,
where the new username may not be correctly reflected in casdoor's Name field
(which is casdoor's unique identifier).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadConfig()
		if err != nil {
			return err
		}
		ctx := cmd.Context()

		if cfg.SSOType != rpc.SSOTypeCasdoor {
			return errors.New("sso type is not casdoor, cannot fix casdoor username")
		}

		certData, err := os.ReadFile(cfg.Casdoor.Certificate)
		if err != nil {
			return fmt.Errorf("failed to read casdoor certificate: %w", err)
		}
		casClient := casdoorsdk.NewClientWithConf(&casdoorsdk.AuthConfig{
			Endpoint:         cfg.Casdoor.Endpoint,
			ClientId:         cfg.Casdoor.ClientID,
			ClientSecret:     cfg.Casdoor.ClientSecret,
			Certificate:      string(certData),
			OrganizationName: cfg.Casdoor.OrganizationName,
			ApplicationName:  cfg.Casdoor.ApplicationName,
		})

		userStore := database.NewUserStore()
		users, err := userStore.Index(ctx)
		if err != nil {
			return fmt.Errorf("failed to index users from database: %w", err)
		}

		var (
			joinErr error
			fixed   int
			skipped int
			failed  int
		)
		for _, user := range users {
			// only fix users that were registered through casdoor
			if user.RegProvider != rpc.SSOTypeCasdoor {
				continue
			}

			casUser, getErr := casClient.GetUserByUserId(user.UUID)
			if getErr != nil {
				slog.Error("failed to get casdoor user by uuid",
					slog.String("username", user.Username),
					slog.String("uuid", user.UUID),
					slog.String("error", getErr.Error()),
				)
				joinErr = errors.Join(joinErr, getErr)
				failed++
				continue
			}
			if casUser == nil {
				slog.Warn("casdoor user not found, skipping",
					slog.String("username", user.Username),
					slog.String("uuid", user.UUID),
				)
				skipped++
				continue
			}

			if casUser.Name == user.Username {
				skipped++
				continue
			}

			slog.Info("fixing inconsistent casdoor user Name",
				slog.String("username", user.Username),
				slog.String("uuid", user.UUID),
				slog.String("casdoor_old_name", casUser.Name),
				slog.String("casdoor_display_name", casUser.DisplayName),
			)

			// Save the original casdoor user identifier (owner/oldName) before
			// modifying the Name field, because GetId() returns "owner/Name".
			// We must look up the user by the OLD name, then apply the new name.
			oldID := casUser.GetId()
			casUser.Name = user.Username
			_, updateErr := casClient.UpdateUserById(oldID, casUser)
			if updateErr != nil {
				slog.Error("failed to update casdoor user Name",
					slog.String("username", user.Username),
					slog.String("uuid", user.UUID),
					slog.String("error", updateErr.Error()),
				)
				joinErr = errors.Join(joinErr, updateErr)
				failed++
				continue
			}
			slog.Info("casdoor user Name fixed", slog.String("username", user.Username))
			fixed++
		}

		slog.Info("fix casdoor username done",
			slog.Int("total", len(users)),
			slog.Int("fixed", fixed),
			slog.Int("skipped", skipped),
			slog.Int("failed", failed),
		)
		return joinErr
	},
}
