package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		fmt.Print(" [up migration] update_agent_knowledge_base_namespace_and_type")
		_, err := db.ExecContext(ctx, `
			ALTER TABLE agent_knowledge_bases
				DROP CONSTRAINT idx_agent_knowledge_bases_user_uuid_name;
			ALTER TABLE agent_knowledge_bases
				ADD COLUMN ns_uuid VARCHAR;
			UPDATE agent_knowledge_bases AS akb
				SET ns_uuid = (
					SELECT ns.uuid
					FROM namespaces AS ns
					JOIN users AS u ON u.id = ns.user_id
					WHERE ns.namespace_type = 'user'
						AND ns.deleted_at IS NULL
						AND u.deleted_at IS NULL
						AND u.uuid = akb.user_uuid
					ORDER BY ns.id
					LIMIT 1
				)
				WHERE EXISTS (
					SELECT 1
					FROM namespaces AS ns
					JOIN users AS u ON u.id = ns.user_id
					WHERE ns.namespace_type = 'user'
						AND ns.deleted_at IS NULL
						AND u.deleted_at IS NULL
						AND u.uuid = akb.user_uuid
				);
			UPDATE agent_knowledge_bases SET ns_uuid = user_uuid WHERE ns_uuid IS NULL;
			ALTER TABLE agent_knowledge_bases
				ALTER COLUMN ns_uuid SET NOT NULL;
			ALTER TABLE agent_knowledge_bases
				ADD COLUMN namespace_type VARCHAR NOT NULL DEFAULT 'user';
			ALTER TABLE agent_knowledge_bases
				ADD COLUMN type VARCHAR NOT NULL DEFAULT 'langflow';
			ALTER TABLE agent_knowledge_bases
				ADD CONSTRAINT idx_agent_knowledge_bases_ns_uuid_name UNIQUE (ns_uuid, name);
		`)
		if err != nil {
			return fmt.Errorf("update agent knowledge base namespace and type: %w", err)
		}
		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		fmt.Print(" [down migration] update_agent_knowledge_base_namespace_and_type")
		_, err := db.ExecContext(ctx, `
			ALTER TABLE agent_knowledge_bases
				DROP CONSTRAINT idx_agent_knowledge_bases_ns_uuid_name;
			ALTER TABLE agent_knowledge_bases
				ADD CONSTRAINT idx_agent_knowledge_bases_user_uuid_name UNIQUE (user_uuid, name);
			ALTER TABLE agent_knowledge_bases DROP COLUMN type;
			ALTER TABLE agent_knowledge_bases DROP COLUMN namespace_type;
			ALTER TABLE agent_knowledge_bases DROP COLUMN ns_uuid;
		`)
		if err != nil {
			return fmt.Errorf("rollback agent knowledge base namespace and type: %w", err)
		}
		return nil
	})
}
