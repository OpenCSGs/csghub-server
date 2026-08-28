//go:build !ee && !saas

package component

import (
	"context"
	"time"

	"opencsg.com/csghub-server/common/types"
)

// invalidateLicenseCaches clears the cached license status after a license
// mutation. CE has no feature-provider reader cache, so only the component
// cache needs clearing.
func invalidateLicenseCaches(c *cachedLicenseComponent) {
	c.cache.Delete(licenseCacheKey)
}

func (c *cachedLicenseComponent) GetLicenseStatusInternal(ctx context.Context) (*types.LicenseStatusResp, error) {
	if cached, found := c.cache.Get(licenseCacheKey); found {
		if cached == nil {
			return nil, nil
		}
		return cached.(*types.LicenseStatusResp), nil
	}

	status, err := c.inner.GetLicenseStatusInternal(ctx)
	if err != nil {
		return nil, err
	}

	if status == nil {
		c.cache.Set(licenseCacheKey, nil, licenseCacheExpire)
		return nil, nil
	}

	ttl := time.Until(status.ExpireTime)
	if ttl <= 0 {
		return status, nil
	}
	if ttl > licenseCacheExpire {
		ttl = licenseCacheExpire
	}
	c.cache.Set(licenseCacheKey, status, ttl)
	return status, nil
}
