//go:build saas

package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const accountMeteringsPartitionedEventUUIDIndex = "idx_account_meterings_partitioned_event_uuid"

const accountMeteringsPartitionedEventUUIDIndexMaxPasses = 5

type accountMeteringsPartition struct {
	SchemaName string `bun:"schema_name"`
	TableName  string `bun:"table_name"`
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		fmt.Print(" [up migration] ")
		return ensureAccountMeteringsPartitionedEventUUIDIndex(ctx, db)
	}, func(ctx context.Context, db *bun.DB) error {
		fmt.Print(" [down migration] ")
		if _, err := db.NewRaw(`DROP INDEX IF EXISTS ?`, bun.Ident(accountMeteringsPartitionedEventUUIDIndex)).Exec(ctx); err != nil {
			return fmt.Errorf("drop account_meterings partitioned event_uuid index: %w", err)
		}
		return nil
	})
}

func ensureAccountMeteringsPartitionedEventUUIDIndex(ctx context.Context, db *bun.DB) error {
	var parentKind string
	if err := db.NewRaw(`SELECT COALESCE((
			SELECT relkind::text FROM pg_class WHERE oid = to_regclass('account_meterings_partitioned')
		), '')`).Scan(ctx, &parentKind); err != nil {
		return fmt.Errorf("inspect account_meterings_partitioned: %w", err)
	}
	if parentKind != "p" {
		return fmt.Errorf("account_meterings_partitioned must be a partitioned table, got relkind %q", parentKind)
	}

	for pass := 1; pass <= accountMeteringsPartitionedEventUUIDIndexMaxPasses; pass++ {
		partitions, err := listAccountMeteringsPartitions(ctx, db)
		if err != nil {
			return err
		}

		for _, partition := range partitions {
			if err := ensureAccountMeteringsPartitionEventUUIDIndex(ctx, db, partition); err != nil {
				return err
			}
		}

		if _, err := db.NewRaw(`CREATE INDEX IF NOT EXISTS ? ON ONLY account_meterings_partitioned (event_uuid)`,
			bun.Ident(accountMeteringsPartitionedEventUUIDIndex)).Exec(ctx); err != nil {
			return fmt.Errorf("create account_meterings partitioned event_uuid index: %w", err)
		}
		for _, partition := range partitions {
			if err := attachAccountMeteringsPartitionEventUUIDIndex(ctx, db, partition); err != nil {
				return err
			}
		}

		valid, err := accountMeteringsPartitionedEventUUIDIndexValid(ctx, db)
		if err != nil {
			return err
		}
		if valid {
			return nil
		}
	}

	return fmt.Errorf("account_meterings partitioned event_uuid index is still invalid after %d passes; partitions may be changing concurrently", accountMeteringsPartitionedEventUUIDIndexMaxPasses)
}

func listAccountMeteringsPartitions(ctx context.Context, db *bun.DB) ([]accountMeteringsPartition, error) {
	var partitions []accountMeteringsPartition
	if err := db.NewRaw(`
			SELECT namespace.nspname AS schema_name, child.relname AS table_name
			FROM pg_partition_tree('account_meterings_partitioned'::regclass) AS tree
			JOIN pg_class AS child ON child.oid = tree.relid
			JOIN pg_namespace AS namespace ON namespace.oid = child.relnamespace
			WHERE tree.isleaf
			ORDER BY namespace.nspname, child.relname
		`).Scan(ctx, &partitions); err != nil {
		return nil, fmt.Errorf("list account_meterings partitions: %w", err)
	}
	return partitions, nil
}

func ensureAccountMeteringsPartitionEventUUIDIndex(ctx context.Context, db *bun.DB, partition accountMeteringsPartition) error {
	indexName := accountMeteringsPartitionEventUUIDIndexName(partition.TableName)
	qualifiedIndexName := partition.SchemaName + "." + indexName
	qualifiedTableName := partition.SchemaName + "." + partition.TableName

	var exists, valid bool
	if err := db.NewRaw(`SELECT to_regclass(?) IS NOT NULL`, qualifiedIndexName).Scan(ctx, &exists); err != nil {
		return fmt.Errorf("inspect event_uuid index existence for %s: %w", qualifiedTableName, err)
	}
	if exists {
		if err := db.NewRaw(`SELECT COALESCE((
			SELECT indisvalid
			FROM pg_index
			WHERE indexrelid = to_regclass(?)
			  AND indrelid = to_regclass(?)
			  AND indnkeyatts = 1
			  AND indkey[0] = (
				SELECT attnum FROM pg_attribute WHERE attrelid = to_regclass(?) AND attname = 'event_uuid'
			  )
		), false)`, qualifiedIndexName, qualifiedTableName, qualifiedTableName).Scan(ctx, &valid); err != nil {
			return fmt.Errorf("inspect event_uuid index validity for %s: %w", qualifiedTableName, err)
		}
		if !valid {
			if _, err := db.NewRaw(`DROP INDEX CONCURRENTLY ?`, bun.Ident(qualifiedIndexName)).Exec(ctx); err != nil {
				return fmt.Errorf("drop unusable event_uuid index for %s: %w", qualifiedTableName, err)
			}
		}
	}
	if !valid {
		if _, err := db.NewRaw(`CREATE INDEX CONCURRENTLY ? ON ? (event_uuid)`,
			bun.Ident(indexName), bun.Ident(qualifiedTableName)).Exec(ctx); err != nil {
			return fmt.Errorf("create event_uuid index on %s: %w", qualifiedTableName, err)
		}
	}
	return nil
}

func attachAccountMeteringsPartitionEventUUIDIndex(ctx context.Context, db *bun.DB, partition accountMeteringsPartition) error {
	indexName := accountMeteringsPartitionEventUUIDIndexName(partition.TableName)
	qualifiedIndexName := partition.SchemaName + "." + indexName
	var attached bool
	if err := db.NewRaw(`SELECT EXISTS (
				SELECT 1 FROM pg_inherits
				WHERE inhparent = to_regclass(?) AND inhrelid = to_regclass(?)
			)`, accountMeteringsPartitionedEventUUIDIndex, qualifiedIndexName).Scan(ctx, &attached); err != nil {
		return fmt.Errorf("inspect event_uuid index attachment for %s.%s: %w", partition.SchemaName, partition.TableName, err)
	}
	if attached {
		return nil
	}
	if _, err := db.NewRaw(`ALTER INDEX ? ATTACH PARTITION ?`,
		bun.Ident(accountMeteringsPartitionedEventUUIDIndex), bun.Ident(qualifiedIndexName)).Exec(ctx); err != nil {
		return fmt.Errorf("attach event_uuid index for %s.%s: %w", partition.SchemaName, partition.TableName, err)
	}
	return nil
}

func accountMeteringsPartitionedEventUUIDIndexValid(ctx context.Context, db *bun.DB) (bool, error) {
	var valid bool
	if err := db.NewRaw(`SELECT COALESCE((
		SELECT indisvalid FROM pg_index WHERE indexrelid = to_regclass(?)
	), false)`, accountMeteringsPartitionedEventUUIDIndex).Scan(ctx, &valid); err != nil {
		return false, fmt.Errorf("inspect account_meterings partitioned event_uuid index validity: %w", err)
	}
	return valid, nil
}

func accountMeteringsPartitionEventUUIDIndexName(tableName string) string {
	const maxPostgresIdentifierLength = 63
	name := "idx_" + tableName + "_event_uuid"
	if len(name) > maxPostgresIdentifierLength {
		return name[:maxPostgresIdentifierLength]
	}
	return name
}
