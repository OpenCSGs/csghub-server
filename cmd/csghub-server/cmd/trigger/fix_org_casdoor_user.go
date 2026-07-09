package trigger

import (
	"errors"
	"log/slog"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
)

var fixOrgCasdoorUserCmd = &cobra.Command{
	Use:   "fix-org-casdoor-user",
	Short: "scan all organizations and create missing casdoor users",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadConfig()
		if err != nil {
			return err
		}
		ctx := cmd.Context()

		if cfg.SSOType != rpc.SSOTypeCasdoor {
			return errors.New("sso type is not casdoor, cannot fix org casdoor user")
		}

		sso, err := rpc.NewSSOClient(cfg)
		if err != nil {
			return err
		}

		orgStore := database.NewOrgStore()
		orgs, _, err := orgStore.GetUserOwnOrgs(ctx, "")
		if err != nil {
			return err
		}

		var (
			joinErr error
			created int
			skipped int
			failed  int
		)
		for _, org := range orgs {
			exists, checkErr := sso.IsExistByName(ctx, org.Name)
			if checkErr != nil {
				slog.Error("check casdoor user exist failed",
					slog.String("org", org.Name),
					slog.String("error", checkErr.Error()),
				)
				joinErr = errors.Join(joinErr, checkErr)
				failed++
				continue
			}
			if exists {
				skipped++
				continue
			}

			createErr := sso.CreateUser(ctx, &rpc.SSOCreateUserInfo{
				Name:     org.Name,
				Nickname: org.Nickname,
				UUID:     org.UUID.String(),
				Password: uuid.New().String(),
			})
			if createErr != nil {
				slog.Error("create casdoor user failed",
					slog.String("org", org.Name),
					slog.String("error", createErr.Error()),
				)
				joinErr = errors.Join(joinErr, createErr)
				failed++
				continue
			}
			slog.Info("casdoor user created", slog.String("org", org.Name))
			created++
		}

		slog.Info("fix org casdoor user done",
			slog.Int("created", created),
			slog.Int("skipped", skipped),
			slog.Int("failed", failed),
		)
		return joinErr
	},
}
