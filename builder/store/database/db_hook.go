//go:build !ee && !saas

package database

import "github.com/uptrace/bun"

func registerDatabaseHooks(_ *bun.DB) {}
