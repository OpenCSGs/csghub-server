package database

import "context"

type ApiRateLimitStore interface {
	Create(ctx context.Context, limit *ApiRateLimit) (err error)
	FindByPath(ctx context.Context, path string) (limit ApiRateLimit, err error)
	FindByID(ctx context.Context, id int64) (limit ApiRateLimit, err error)
	Delete(ctx context.Context, path string) (err error)
	DeleteByID(ctx context.Context, id int64) (err error)
	Update(ctx context.Context, limit *ApiRateLimit) (err error)
	UpdateByID(ctx context.Context, limit *ApiRateLimit) (err error)
	List(ctx context.Context) (limits []ApiRateLimit, err error)
}

type ApiRateLimit struct {
	ID      int64  `bun:",pk,autoincrement" json:"id"`
	Path    string `bun:",notnull,unique" json:"path"`
	Limit   int64  `bun:",notnull" json:"limit"`
	Window  int64  `bun:",notnull" json:"window"`
	CheckIP bool   `bun:",notnull,default:false" json:"checkIP"`
	times
}
