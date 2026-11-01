SET statement_timeout = 0;

--bun:split

-- Allow runtime agent instances to carry an effective_mode/storage_policy pair
-- (read_only/read_write csgclaw agents). Both must be set together or both
-- NULL (legacy rows). agent_instances_mode_storage_check still constrains the
-- allowed mode/policy combinations.
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
            (
                (effective_mode IS NULL AND storage_policy IS NULL) OR
                (effective_mode IS NOT NULL AND storage_policy IS NOT NULL)
            )
        )
    );
