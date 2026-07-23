SET statement_timeout = 0;

--bun:split

DELETE FROM space_resource_scenario_constraints
WHERE scenario = 'wf_dataflow_llmlog';
