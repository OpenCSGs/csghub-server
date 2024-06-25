package component

import (
	"context"
	"fmt"
	"net/http"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/multisync/accounting"
	"opencsg.com/csghub-server/multisync/types"
)

type MirrorProxyComponent struct {
	ac   *accounting.AccountingClient
	user *database.UserStore
}

func NewMirrorProxyComponent(config *config.Config) (*MirrorProxyComponent, error) {
	ac, err := accounting.NewAccountingClient(config)
	if err != nil {
		return nil, err
	}
	return &MirrorProxyComponent{
		ac:   ac,
		user: database.NewUserStore(),
	}, nil
}

func (c *MirrorProxyComponent) Serve(ctx context.Context, req *types.GetSyncQuotaStatementReq) error {
	sq, _, err := c.ac.GetSyncQuota(&accounting.GetSyncQuotaReq{
		AccessToken: req.AccessToken,
	})
	if err != nil {
		return fmt.Errorf("error getting sync quota: %v", err)
	}
	if sq.RepoCountLimit <= sq.RepoCountUsed {
		return fmt.Errorf("sync repository count limit exceeded")
	}
	sqs, _, err := c.ac.GetSyncQuotaStatement(&accounting.GetSyncQuotaStatementsReq{
		AccessToken: req.AccessToken,
		RepoPath:    req.RepoPath,
		RepoType:    req.RepoType,
	})
	if err != nil {
		return fmt.Errorf("error getting sync quota statement: %v", err)
	}
	if sqs != nil {
		return nil
	}
	resp, err := c.ac.CreateSyncQuotaStatement(&accounting.CreateSyncQuotaStatementReq{
		AccessToken: req.AccessToken,
		RepoPath:    req.RepoPath,
		RepoType:    req.RepoType,
	})
	if err != nil {
		return fmt.Errorf("error creating sync quota statement: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error creating sync quota statement")
	}
	return nil
}