//go:build !ee && !saas

package database

import (
	"context"

	"github.com/uptrace/bun"
)

func checkAgentPolicyUpdate(context.Context, bun.Tx, *AgentInstance, *map[string]any) error {
	return nil
}

func lockRecoveryAgent(context.Context, bun.Tx, int64) error { return nil }

func updateLogicalAgentDeployTimeouts(context.Context, bun.Tx, int64, *int) error { return nil }
