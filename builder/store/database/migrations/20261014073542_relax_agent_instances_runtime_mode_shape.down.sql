SET statement_timeout = 0;

--bun:split

-- Runtime rows must not carry mode/policy under the original constraint.
UPDATE agent_instances SET effective_mode = NULL, storage_policy = NULL WHERE kind = 'runtime';

ALTER TABLE agent_instances
    DROP CONSTRAINT agent_instances_kind_shape_check,
    ADD CONSTRAINT agent_instances_kind_shape_check CHECK (
        (
            kind = 'logical' AND
            effective_mode IS NOT NULL AND storage_policy IS NOT NULL AND
            blueprint IS NOT NULL AND blueprint_sha256 IS NOT NULL
        ) OR
        (
            kind = 'runtime' AND deletion_started_at IS NULL AND
            blueprint IS NULL AND blueprint_sha256 IS NULL AND
            effective_mode IS NULL AND storage_policy IS NULL
        )
    );
