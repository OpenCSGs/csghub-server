SET statement_timeout = 0;

--bun:split

DROP TABLE IF EXISTS agent_guest_usage_counters;

DROP TABLE IF EXISTS agent_execution_leases;

DROP TABLE IF EXISTS agent_execution_admissions;

DROP TABLE IF EXISTS agent_allocation_sessions;

DROP TABLE IF EXISTS agent_capacity_reservations;
DROP TABLE IF EXISTS agent_capacity_counters;
DROP TABLE IF EXISTS agent_allocation_operations;
DROP TABLE IF EXISTS agent_allocations;

DROP INDEX IF EXISTS agent_instances_live_owner_kind_name;

ALTER TABLE agent_instances
    DROP CONSTRAINT IF EXISTS agent_instances_kind_shape_check,
    DROP CONSTRAINT IF EXISTS agent_instances_mode_storage_check;

ALTER TABLE agent_instances
    DROP CONSTRAINT IF EXISTS agent_instances_storage_policy_check,
    DROP CONSTRAINT IF EXISTS agent_instances_mode_check,
    DROP CONSTRAINT IF EXISTS agent_instances_kind_check,
    DROP COLUMN IF EXISTS blueprint_sha256,
    DROP COLUMN IF EXISTS blueprint,
    DROP COLUMN IF EXISTS storage_policy,
    DROP COLUMN IF EXISTS effective_mode,
    DROP COLUMN IF EXISTS deletion_started_at,
    DROP COLUMN IF EXISTS kind;
