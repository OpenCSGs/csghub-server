//go:build !saas

package database

import "context"

type apiRateLimitStoreImpl struct {
}

func NewApiRateLimitStore() ApiRateLimitStore {
	return &apiRateLimitStoreImpl{}
}

func (s *apiRateLimitStoreImpl) Create(ctx context.Context, limit *ApiRateLimit) (err error) {
	return err
}

func (s *apiRateLimitStoreImpl) FindByPath(ctx context.Context, path string) (limit ApiRateLimit, err error) {
	return
}

func (s *apiRateLimitStoreImpl) Delete(ctx context.Context, path string) (err error) {
	return
}

func (s *apiRateLimitStoreImpl) Update(ctx context.Context, limit *ApiRateLimit) (err error) {
	return
}

func (s *apiRateLimitStoreImpl) List(ctx context.Context) (limits []ApiRateLimit, err error) {
	return
}

func (s *apiRateLimitStoreImpl) FindByID(ctx context.Context, id int64) (limit ApiRateLimit, err error) {
	return
}

func (s *apiRateLimitStoreImpl) DeleteByID(ctx context.Context, id int64) (err error) {
	return
}

func (s *apiRateLimitStoreImpl) UpdateByID(ctx context.Context, limit *ApiRateLimit) (err error) {
	return
}
