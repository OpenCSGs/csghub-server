package component

import (
	"context"
	"sync"
	"time"

	"github.com/patrickmn/go-cache"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

const (
	licenseCacheKey    = "license"
	licenseCacheExpire = 5 * time.Minute
)

var (
	sharedLicenseCacheMu         sync.Mutex
	sharedLicenseComponentByBase = map[any]LicenseComponent{}
)

// cachedLicenseComponent wraps a LicenseComponent and caches the result of
// GetLicenseStatusInternal for the license middleware and handler status reads.
// sharedKey ties this instance to the feature provider's reader cache created
// under the same key so a license mutation can invalidate both in-process.
type cachedLicenseComponent struct {
	inner     LicenseComponent
	cache     *cache.Cache
	sharedKey any
}

// NewCachedLicenseComponent returns a LicenseComponent that caches the active
// license status for 5 minutes, clamping the TTL to the license expiry so an
// expired license is never served from cache.
func NewCachedLicenseComponent(inner LicenseComponent) LicenseComponent {
	return &cachedLicenseComponent{
		inner: inner,
		cache: cache.New(licenseCacheExpire, 10*time.Minute),
	}
}

func NewSharedCachedLicenseComponent(inner LicenseComponent) LicenseComponent {
	return NewSharedCachedLicenseComponentWithKey(inner, inner)
}

func NewSharedCachedLicenseComponentWithKey(key any, inner LicenseComponent) LicenseComponent {
	sharedLicenseCacheMu.Lock()
	defer sharedLicenseCacheMu.Unlock()
	if cached, ok := sharedLicenseComponentByBase[key]; ok {
		return cached
	}
	cached := &cachedLicenseComponent{
		inner:     inner,
		cache:     cache.New(licenseCacheExpire, 10*time.Minute),
		sharedKey: key,
	}
	sharedLicenseComponentByBase[key] = cached
	return cached
}

func (c *cachedLicenseComponent) ListLicense(ctx context.Context, req types.QueryLicenseReq) ([]database.License, int, error) {
	return c.inner.ListLicense(ctx, req)
}

func (c *cachedLicenseComponent) CreateLicense(ctx context.Context, req *types.CreateLicenseReq) (string, error) {
	license, err := c.inner.CreateLicense(ctx, req)
	if err == nil {
		invalidateLicenseCaches(c)
	}
	return license, err
}

func (c *cachedLicenseComponent) ImportLicense(ctx context.Context, req types.ImportLicenseReq) error {
	err := c.inner.ImportLicense(ctx, req)
	if err == nil {
		invalidateLicenseCaches(c)
	}
	return err
}

func (c *cachedLicenseComponent) GetLicenseByID(ctx context.Context, req types.GetLicenseReq) (*database.License, string, error) {
	return c.inner.GetLicenseByID(ctx, req)
}

func (c *cachedLicenseComponent) UpdateLicense(ctx context.Context, id int64, req *types.UpdateLicenseReq) (*database.License, error) {
	license, err := c.inner.UpdateLicense(ctx, id, req)
	if err == nil {
		invalidateLicenseCaches(c)
	}
	return license, err
}

func (c *cachedLicenseComponent) GetLicenseStatus(ctx context.Context, req types.LicenseStatusReq) (*types.LicenseStatusResp, error) {
	return c.inner.GetLicenseStatus(ctx, req)
}

func (c *cachedLicenseComponent) DeleteLicenseByID(ctx context.Context, id int64, currentUser string) error {
	err := c.inner.DeleteLicenseByID(ctx, id, currentUser)
	if err == nil {
		invalidateLicenseCaches(c)
	}
	return err
}

func (c *cachedLicenseComponent) VerifyLicense(ctx context.Context, req types.ImportLicenseReq) (*types.RSAPayload, error) {
	return c.inner.VerifyLicense(ctx, req)
}

// InvalidateLicenseCache clears the cached license status. It can be called
// directly when the license state changes outside of the component methods.
func (c *cachedLicenseComponent) InvalidateLicenseCache() {
	invalidateLicenseCaches(c)
}

var _ LicenseComponent = (*cachedLicenseComponent)(nil)
