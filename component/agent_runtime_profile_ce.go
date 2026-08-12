//go:build !ee && !saas

package component

import "context"

func InitAgentRuntimeProfiles(ctx context.Context) error {
	return initAgentRuntimeProfiles(ctx)
}
