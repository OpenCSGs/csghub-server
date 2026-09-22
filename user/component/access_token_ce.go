//go:build !saas && !ee

package component

import (
	"context"
	"fmt"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func (c *accessTokenComponentImpl) buildNewAccessTokenQuotas(key *database.AccessToken, req *types.CreateUserTokenRequest) ([]*database.AccountAccessTokenQuota, error) {
	if req == nil || len(req.Quotas) < 1 {
		return []*database.AccountAccessTokenQuota{
			{
				APIKey:    key.Token,
				QuotaType: types.AccountingQuotaTypeUnlimited,
				ValueType: types.AccountingQuotaValueTypeFee,
			},
		}, nil
	}
	quotas := make([]*database.AccountAccessTokenQuota, 0, len(req.Quotas))
	for _, item := range req.Quotas {
		quota := &database.AccountAccessTokenQuota{
			APIKey:     key.Token,
			QuotaType:  item.QuotaType,
			ValueType:  item.ValueType,
			Usage:      0,
			Quota:      item.Quota,
			Allocation: item.Allocation,
		}
		quotas = append(quotas, quota)
	}
	return quotas, nil
}

func (c *accessTokenComponentImpl) updateAccessTokenQuotas(ctx context.Context, key *database.AccessToken, items []types.UpdateAPIKeyQuotaItem) ([]*database.AccountAccessTokenQuota, error) {
	if len(items) == 0 {
		return nil, nil
	}

	existingQuotas, err := c.tokenQuotaStore.FindByTokenID(ctx, key.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to find api key quotas by token id, error:%w", err)
	}

	quotas := make([]*database.AccountAccessTokenQuota, 0, len(items))
	for _, item := range items {
		var quota *database.AccountAccessTokenQuota
		for i := range existingQuotas {
			if existingQuotas[i].ID == item.ID {
				quota = &existingQuotas[i]
				break
			}
		}

		if quota == nil {
			quota = &database.AccountAccessTokenQuota{
				APIKey:     key.Token,
				TokenID:    key.ID,
				QuotaType:  item.QuotaType,
				ValueType:  item.ValueType,
				Usage:      0,
				Quota:      item.Quota,
				Allocation: item.Allocation,
			}
		} else {
			quota.QuotaType = item.QuotaType
			quota.ValueType = item.ValueType
			quota.Quota = item.Quota
			quota.Allocation = item.Allocation
		}
		quotas = append(quotas, quota)
	}
	return quotas, nil
}

func (c *accessTokenComponentImpl) GetAPIKeyQuotas(ctx context.Context, apiKey string) ([]database.AccountAccessTokenQuota, error) {
	return []database.AccountAccessTokenQuota{}, nil
}
