//go:build !saas && !ee

package component

import (
	"context"
	"fmt"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func (c *accessTokenComponentImpl) buildNewAccessTokenQuota(ctx context.Context, key *database.AccessToken, req *types.CreateUserTokenRequest) (*database.AccountAccessTokenQuota, error) {
	quota := &database.AccountAccessTokenQuota{
		APIKey:    key.Token,
		QuotaType: types.AccountingQuotaTypeUnlimited,
		ValueType: types.AccountingQuotaValueTypeFee,
		Usage:     0,
		Quota:     0,
	}
	return quota, nil
}

func (c *accessTokenComponentImpl) updateAccessTokenQuota(ctx context.Context, key *database.AccessToken, req *types.UpdateAPIKeyRequest) (*database.AccountAccessTokenQuota, error) {
	quotas, err := c.tokenQuotaStore.FindByAPIKey(ctx, key.Token)
	if err != nil {
		return nil, fmt.Errorf("fail to find api key quota,error:%w", err)
	}
	var quota *database.AccountAccessTokenQuota
	if len(quotas) == 0 {
		quota = &database.AccountAccessTokenQuota{
			APIKey:    key.Token,
			QuotaType: types.AccountingQuotaTypeUnlimited,
			ValueType: types.AccountingQuotaValueTypeFee,
			Usage:     0,
			Quota:     0,
		}
		err := c.tokenQuotaStore.Create(ctx, quota)
		if err != nil {
			return nil, fmt.Errorf("fail to create api key quota, error:%w", err)
		}
	} else {
		quota = &quotas[0]
	}
	return quota, nil
}
