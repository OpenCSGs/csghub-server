package member

import (
	"context"
	"fmt"

	"opencsg.com/starhub-server/builder/gitserver"
	"opencsg.com/starhub-server/builder/store/database"
	"opencsg.com/starhub-server/common/config"
)

type Controller struct {
	memberStore *database.MemberStore
	orgStore    *database.OrgStore
	gitServer   gitserver.GitServer
}

func New(config *config.Config) (*Controller, error) {
	dbConfig := database.DBConfig{
		Dialect: database.DatabaseDialect(config.Database.Driver),
		DSN:     config.Database.DSN,
	}

	db, err := database.NewDB(context.Background(), dbConfig)
	if err != nil {
		return nil, err
	}

	gs, err := gitserver.NewGitServer(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create gitserver:%w", err)
	}
	return &Controller{
		memberStore: database.NewMemberStore(db),
		orgStore:    database.NewOrgStore(db),
		gitServer:   gs,
	}, nil
}
