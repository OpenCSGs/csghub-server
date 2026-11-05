CSGHub server supports two configuration methods: using environment variables or configuration files. You can also combine them, for example, by placing non-sensitive configurations in a config file and overriding sensitive configurations with environment variables.

### Using a Config File

CSGHub supports TOML format for config files. When starting any service from the command line, you can specify the config file with the `--config` option:

```
go run cmd/csghub-server/main.go start server --config local.toml
go run cmd/csghub-server/main.go deploy runner --config local.toml
go run cmd/csghub-server/main.go mirror repo-sync --config common/config/test.toml
```

We provide an [example config file](common/config/config.toml.example), you can rename it, modify as needed and use. All available configurations are defined in [this Go file](common/config/config.go). The TOML configuration uses snake_case naming convention, and names automatically map to corresponding struct field names.

If a config value is missing in the config file, the `default` tag value specified in the Go struct file will be used.

### Using Environment Variables

CSGHub also supports configuration through environment variables. The relevant environment variable names are defined in [this Go file](common/config/config.go) under the `env` tag. If an environment variable is absent, the value in the `default` tag will be used.

### Combining Config File with Environment Variables

You can use config file together with environment variables. When both are used, environment variables take **higher priority** than the config file. For example, if you have a `Port` setting, and you specify it in the TOML file as `port=1234` and in an environment variable as `export PORT=5678`, the environment variable value (5678) will be used for the port configuration.

### Organization Mode

`STARHUB_SERVER_ORGANIZATION_ENABLE_UNIT` enables hierarchy mode in EE/SaaS.
CE always uses single-level organizations, even when this setting is `true`.
Routes and organization stores use `Config.IsHierarchicalOrganization()` to
resolve the effective mode consistently. A nil configuration uses single-level mode.

### Transactional Tuple Worker

Permission changes made by ordinary business requests are saved to River on the
same SQL transaction as their business rows. The User service consumes these
events through its dedicated River client:

```sh
./starhub user launch --config /path/to/config.toml
```

User runs two leased clients for `organization_deletion` and
`rebac_tuple_queue`. Each queue has four workers per User instance; tuple concurrency
is fixed and has no configuration option. No separate tuple command or deployment
is required.
The User service must use the same database and OpenFGA store as the producers.
API, sync and other services enqueue events without calling User over HTTP.
Business component constructors create the transactional producer and inject it
into tuple-changing Stores. The DB handle contains no job client. Temporal
activities and Glama mirror creation use the same component constructors, so their
Stores receive the producer without command-specific setup.
Read-only Stores can omit the producer; a tuple-changing operation without one
returns an error and rolls back its business transaction.

User startup begins River consumption before opening its HTTP listener. API and
User startup have no additional migration-job waiting logic. Migrations retain
their existing synchronous OpenFGA behavior and do not enqueue tuple events.
The EE organization import command also retains synchronous OpenFGA writes and
best-effort compensation on failure; it is excluded from the tuple queue rollout.
For a fresh install, allow the API migration phase to initialize the River/OpenFGA schemas before starting
User. User downtime pauses tuple consumption; committed events remain in River
and resume after User recovers.

Repository and organization deletion transactions snapshot their permission
removals and enqueue ReBAC jobs alongside the existing cleanup jobs. The large
cleanup workers retain their storage, SSO and SQL responsibilities and do not
call OpenFGA or create tuple child jobs. Their completion is independent of
permission projection completion.
All worker processes register the tuple job kind so River's shared maintenance
services can inspect it safely. User startup and shutdown manage both River clients;
failure to start either client releases both clients before returning an error.
The tuple worker uses the User service's existing authorizer, backed by
the process-wide default OpenFGA provider. Event model IDs remain metadata and do not select an authorizer;
execution uses the provider's configured model.

User direct-grant mutations and account deletion take the same transaction-scoped
PostgreSQL advisory lock before subsequent row locks. This keeps deletion
snapshots current and prevents queued mutations from recreating grants for a
deleted user. Commit and rollback release the lock automatically.

Events contain a version, authorization model ID, source, and immutable ordered
write/delete operations. Each transaction collects its events and inserts one
batch after business mutations finish. Events are grouped by subject type and
ID, with usersets sharing their base subject's sequence. Public `user:*` relations
are additionally grouped by object type and ID, so each knowledge base retains
ordered visibility changes without blocking other knowledge bases. A job contains at most
50 write/delete relationships; larger groups are split into successive jobs in
the same sequence. All chunks commit or roll back with the business transaction.
OpenFGA requests contain at most 50 relationships. Duplicate writes and missing
deletes are idempotent. Normal tuple execution and organization deletion
use River's default retry policy:
the next failure number to the fourth power in seconds, with +/-10% jitter,
derived from persisted error history, up to 12 total attempts. Rescue retry timing
still depends on the leader's registered worker policy. The existing workhub
client retains discarded jobs indefinitely. Monitor the
`rebac_tuple_queue` queue's oldest pending job, retry counts and discarded jobs;
cancelled or discarded events are skipped by their subject sequence. Their
unapplied or partially applied permission changes require investigation because
later jobs do not necessarily repair them. Do not delete unapplied events as cleanup.

The pinned River fork provides subject ordering across retries and User replicas.
Only ReBAC jobs opt into sequences; mirror, LFS and large cleanup jobs retain
ordinary scheduling. River's default reconcilers may run in multiple worker
processes, while only User consumes the ReBAC queue. Retries and snoozes keep
successors waiting. Both `ContinueOnCancelled` and `ContinueOnDiscarded` are true:
cancellation or exhausted attempts release the next member without changing the
failed job's outcome. Other successors still execute in sequence; this does not
enable group-wide bypass. Policies are persisted on insertion, so existing jobs
retain their originally stored policy. Resolved historical jobs cannot be retried
in place; any repair must respect the subject's current state and sequence.
Permission propagation is asynchronous and may lag behind the business response.
A separate post-commit Authorizer.Write/Delete would bypass sequencing and must
not be added.

Organization deletion requests attempt immediate SSO cleanup only. The existing
organization deletion worker retains direct-grant SQL cleanup. Tuple jobs carry
their own immutable identities, so their retries remain possible after those
SQL records disappear. The CE edition retains its existing direct-grant cleanup
boundary.

All work clients use a ten-minute `RescueStuckJobsAfter` for unleased jobs, measured
from the execution's `attempted_at`. The leader also checks the worker's timeout
before rescuing an unleased job. Leased jobs use lease expiration plus the rescue
grace period instead. Scanning and retry scheduling add to the recovery time.

| Job | Execution timeout | Execution leases | Subject sequences |
| --- | --- | --- | --- |
| Repository mirror, normal and urgent | 30 minutes | Enabled | Disabled |
| LFS mirror, normal and urgent | 24 hours | Enabled | Disabled |
| Repository deletion | 10 minutes | Enabled | Disabled |
| Organization deletion | 15 minutes | Enabled | Disabled |
| ReBAC tuple | 30 seconds | Enabled | Enabled |

Leased clients use the fork defaults: a 120-second lease, 20-second heartbeat,
60-second rescue grace period and 30-second rescue scan interval. Renewal does not
extend the execution timeout; cancellation still requires the worker to cooperate.
All leader-eligible processes must use compatible rescue settings.

Drain any queued jobs using the previous shared `user:*` sequence before switching
to per-object wildcard sequences; old and new keys must not execute concurrently
for the same knowledge base.

Apply the River ordering/lease schema migration and upgrade all worker runtimes
sharing the schema before enabling the leased clients. Schema down is an offline
operation and refuses unresolved sequence jobs.

Keep the consumer running until outstanding events are reconciled when rolling
back application binaries. Older binaries that write tuples directly must not
be mixed with uncoordinated outstanding tuple events.
