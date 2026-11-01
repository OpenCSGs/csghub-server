//go:build !ee && !saas

package database

import "context"

func withUserTagAuditSuppressed(ctx context.Context) context.Context {
	return ctx
}
