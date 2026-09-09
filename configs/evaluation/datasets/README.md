# Evaluation dataset rules

Each `*-datasets.json` file is an authoritative set of dataset tag rules for
the evaluation engine named by the file. The `runtime_framework` value must
match the filename.

- `compatible_runtime_frameworks` applies the same rules to compatible
  engines. EvalScope and AMD EvalScope intentionally share one dataset list.
- `prune: true` removes stale rules for each listed framework after upserting
  the configured rules. Empty dataset lists are rejected before synchronization.
- These files synchronize `tag_rules`; they do not mirror dataset repositories.
  Newly added upstream datasets still require the normal repository sync flow
  before they appear in repository-backed UI lists.
