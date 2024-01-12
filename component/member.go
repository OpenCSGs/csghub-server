package component

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/builder/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
)

type MemberComponent struct {
	memberStore *database.MemberStore
	orgStore    *database.OrgStore
	gitServer   gitserver.GitServer
}

func NewMemberComponent(config *config.Config) (*MemberComponent, error) {
	gs, err := gitserver.NewGitServer(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create gitserver:%w", err)
	}
	return &MemberComponent{
		memberStore: database.NewMemberStore(),
		orgStore:    database.NewOrgStore(),
		gitServer:   gs,
	}, nil
}

func (c *MemberComponent) Index(ctx *gin.Context) (members []database.Member, err error) {
	return
}

func (c *MemberComponent) Create(ctx *gin.Context) (org *database.Member, err error) {
	return
}

func (c *MemberComponent) Update(ctx *gin.Context) (org *database.Member, err error) {
	return
}

func (c *MemberComponent) Delete(ctx *gin.Context) (err error) {
	return
}
