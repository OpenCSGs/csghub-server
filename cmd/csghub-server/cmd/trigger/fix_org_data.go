package trigger

import (
	"context"
	"errors"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/user/component"
)

var fixOrgDataCmd = &cobra.Command{
	Use:   "fix-org-data",
	Short: "scan organization and fix organization data, like init read,write and admin role for org",
	RunE: func(cmd *cobra.Command, args []string) error {
		lh := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			AddSource: false,
			Level:     slog.LevelInfo,
		})
		l := slog.New(lh)
		slog.SetDefault(l)

		var orgs []database.Organization
		var err error
		var cfg *config.Config
		cfg, err = config.LoadConfig()
		if err != nil {
			return err
		}
		ctx := context.Background()
		os := database.NewOrgStore()
		orgComponent, _ := component.NewOrganizationComponent(cfg)

		// get all organizations
		orgs, err = os.GetUserOwnOrgs(ctx, "")
		for _, org := range orgs {
			req := new(types.CreateOrgReq)
			req.Name = org.Name
			req.Nickname = org.Nickname
			req.Username = org.User.Username
			req.Description = org.Description

			slog.Info("before create org", slog.Any("req", req))
			_, err1 := orgComponent.FixOrgData(ctx, &org)
			if err1 != nil {
				err = errors.Join(err, err1)
				slog.Error("create org has error", slog.String("error", err.Error()))
			}
			slog.Info("done create org", slog.String("org", req.Name))
		}
		return err
	},
}
