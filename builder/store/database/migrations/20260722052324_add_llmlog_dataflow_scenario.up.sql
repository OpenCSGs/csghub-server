SET statement_timeout = 0;

--bun:split

INSERT INTO space_resource_scenario_constraints (
    scenario,
    code,
    category,
    i18n_key,
    required_hardware,
    exclude_hardware,
    max_replica
)
VALUES (
    'wf_dataflow_llmlog',
    39,
    'workflow',
    'scenario.wf_dataflow_llmlog',
    0,
    0,
    1
)
ON CONFLICT (scenario) DO UPDATE SET
    code = EXCLUDED.code,
    category = EXCLUDED.category,
    i18n_key = EXCLUDED.i18n_key,
    required_hardware = EXCLUDED.required_hardware,
    exclude_hardware = EXCLUDED.exclude_hardware,
    max_replica = EXCLUDED.max_replica,
    updated_at = CURRENT_TIMESTAMP;
