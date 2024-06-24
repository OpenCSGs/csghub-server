package component

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"opencsg.com/csghub-server/accounting/types"
	"opencsg.com/csghub-server/builder/store/database"
	commontypes "opencsg.com/csghub-server/common/types"
)

type AccountingSyncQuotaStatementComponent struct {
	asq  *database.AccountSyncQuotaStatementStore
	user *database.UserStore
	repo *database.RepoStore
}

func NewAccountingQuotaStatementComponent() *AccountingSyncQuotaStatementComponent {
	asqsc := &AccountingSyncQuotaStatementComponent{
		asq:  database.NewAccountSyncQuotaStatementStore(),
		user: database.NewUserStore(),
		repo: database.NewRepoStore(),
	}
	return asqsc
}

func (a *AccountingSyncQuotaStatementComponent) CreateQuotaStatement(ctx context.Context, currentUser string, input types.ACCT_QUOTA_STATEMENT_REQ) (*database.AccountSyncQuotaStatement, error) {
	user, err := a.user.FindByUsername(ctx, currentUser)
	if err != nil {
		return nil, errors.New("user does not exist")
	}

	fields := strings.Split(input.RepoPath, "/")
	if len(fields) != 2 {
		return nil, errors.New("invalid repo")
	}
	r, err := a.repo.FindByPath(ctx, commontypes.RepositoryType(input.RepoType), fields[0], fields[1])
	if err != nil || r == nil {
		return nil, errors.New("repo does not exist")
	}

	acctQuotaSM := database.AccountSyncQuotaStatement{
		UserID:   user.ID,
		RepoPath: input.RepoPath,
		RepoType: input.RepoType,
	}

	err = a.asq.Create(ctx, acctQuotaSM)
	if err != nil {
		return nil, fmt.Errorf("fail to create quota statement, %w", err)
	}
	return &acctQuotaSM, nil
}

func (a *AccountingSyncQuotaStatementComponent) GetQuotaStatement(ctx context.Context, currentUser string, req types.ACCT_QUOTA_STATEMENT_REQ) (*database.AccountSyncQuotaStatement, error) {
	user, err := a.user.FindByUsername(ctx, currentUser)
	if err != nil {
		return nil, errors.New("user does not exist")
	}
	quotaSM, err := a.asq.Get(ctx, user.ID, req)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("fail to get quota statement, %w", err)
	}
	return quotaSM, nil
}
