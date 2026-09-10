# Repository Agent Guidelines

## Scope and Precedence

These instructions apply to the entire repository. If a more specific `AGENTS.md`
exists below the directory being changed, follow both files and let the more
specific instructions take precedence.

Preserve unrelated working-tree changes. Do not modify files outside the scope of
the task, and do not overwrite changes that were already present.

## Required Workflow

1. Inspect the affected implementation, its callers, and nearby tests before
   editing.
2. Preserve the dependency direction `handler -> component -> builder`.
3. Make the smallest behaviorally complete change and reuse existing repository
   patterns where practical.
4. Run `gofmt` on changed Go files. Do not reformat unrelated files.
5. Add or update tests for changed behavior.
6. Run targeted tests first, followed by the relevant build-tag test and lint
   commands when the change warrants them.
7. Report the behavior changed, the affected layers, and the exact validation
   commands and results. If a command cannot run, report the reason.

## Architecture

CSGHub Server is a Go project organized as a set of microservices. Most request
paths use the following layered architecture:

`handler -> component -> builder`

- **Handler** parses and validates transport input, obtains request-scoped
  identity or authorization data, calls the component layer, and maps results to
  HTTP or RPC responses. Keep business rules out of handlers.
- **Component** owns business rules and coordinates dependencies. It must not
  depend on handler-specific types.
- **Builder** implements infrastructure concerns such as database access, Git,
  RPC clients, Kubernetes, and other external systems. It must not depend on the
  component or handler layers.

Interfaces must not expose lower-layer implementation types. For example, a
component interface must not return `database.User`. Define layer-owned DTOs, or
use stable cross-layer types from `common/types` when sharing is genuinely
necessary. Do not use `common/types` as a dumping ground for layer-specific
types.

### Service Map

| Service | Path | Primary responsibility |
|---|---|---|
| API | `api/` | External HTTP API and cross-service request entry point |
| User | `user/` | Registration, authentication, and user profiles |
| Accounting | `accounting/` | Usage accounting and balance updates |
| Moderation | `moderation/` | Text and image content moderation |
| DataViewer | `dataviewer/` | Dataset metadata and file previews |
| Notification | `notification/` | Email and push notifications |
| Payment | `payment/` | Payments and refunds |
| AIGateway | `aigateway/` | External AI inference entry point |
| Runner | `runner/` | Bridge between the API service and Kubernetes deployments |
| LogCollector | `logcollector/` | Collection of logs from Runner and API workloads |

## Coding Conventions

- Follow standard Go conventions and run the repository's formatting and linting
  tools. Use MixedCaps, preserve conventional initialisms such as `ID`, `HTTP`,
  `API`, and `URL`, and start exported identifiers with an uppercase letter.
- Declare variables in the smallest practical scope.
- Prefer guard clauses and clear error propagation over deeply nested control
  flow.
- Use request/result structs for multi-field or evolving layer boundaries. Do
  not introduce wrapper structs for simple scalar values unless they add domain
  meaning.
- Search for an existing implementation or dependency before introducing a new
  abstraction or external dependency.
- Add an external dependency only when the standard library and existing
  dependencies do not reasonably solve the problem. Document why it is needed,
  use standard Go tooling to update `go.mod` and `go.sum`, and include the
  dependency impact in the final report.
- Add or update unit tests for new or changed behavior. Generated files,
  migrations, declaration-only files, and trivial wiring do not require a
  one-to-one `*_test.go` file.

## Testing and Validation

Choose validation in proportion to the change:

- Run focused package tests while iterating, for example:
  `go test ./component/...` or `go test ./api/handler/...`.
- Determine affected editions from filename suffixes and `//go:build`
  constraints. Files without edition constraints are shared unless their
  callers prove otherwise.
- Run `make test GO_TAGS=<ce|ee|saas>` for each affected edition. If the affected
  edition cannot be determined confidently, run `make test_all`.
- Run `make test_all` when shared or build-tagged behavior may affect all
  editions.
- Run `make lint GO_TAGS=<ce|ee|saas>` for each affected edition, or
  `make lint_all` for shared changes.
- Inspect `Makefile` for additional build, test, lint, and generation commands.

Validation expectations by change type:

- Documentation-only changes: inspect the rendered structure when relevant and
  run `git diff --check`; Go tests are not required.
- Localized Go changes: run focused package tests plus tests for each affected
  edition.
- Shared interfaces, build-tagged behavior, or cross-service changes: run
  `make test_all` and, when practical, `make lint_all`.
- Migration changes: run relevant store tests and validate forward and rollback
  behavior against a disposable local database when practical.

Mock external dependencies such as database stores and RPC clients. Keep tests
deterministic and cover relevant success, failure, authorization, and boundary
conditions.

## Generated Code

Do not manually edit generated files. Common generated outputs include:

- `_mocks/**`
- `component/**/wire_gen.go` and `component/**/wire_gen_test.go`
- `docs/docs.go`, `docs/swagger.json`, and `docs/swagger.yaml`
- any file containing a `Code generated ... DO NOT EDIT` header

When a mocked interface changes, update the appropriate mockery configuration if
needed and run:

```sh
make mock_gen
```

The repository uses `.mockery.yaml`, `.mockery_ee.yaml`, and
`.mockery_saas.yaml` for edition-specific mocks. Review generated diffs and run
the affected tests after regeneration.

When Wire output must be refreshed, use `make mock_wire` rather than editing its
generated files.

When API routes or Swagger annotations change, validate that Swagger
documentation can still be generated by running:

```sh
make swag
```

This command is a generation check only. Do not stage, commit, or otherwise
include `docs/docs.go`, `docs/swagger.json`, or `docs/swagger.yaml` in the final
change. CI/CD is responsible for generating the complete Swagger documentation.

## Database Migrations

Never create migration files manually. Generate them with the repository CLI:

```sh
go run cmd/csghub-server/main.go migration create_go <name>
go run cmd/csghub-server/main.go migration create_sql <name>
```

Use `builder/store/database/migrations/20240201061926_create_spaces.go` as a
reference for Go migrations. Preserve the generated timestamp, filename, and
ordering; do not rename migration files or create timestamps by hand. After
generation, implement the migration, review both the up and down paths, and
validate forward and rollback behavior where applicable.

## Reference Implementations

- Routers: `api/router/api.go`, `accounting/router/api.go`,
  `runner/router/api.go`
- Handlers: `api/handler/space.go`, `api/handler/evaluation.go`
- Components: `component/space.go`, `component/evaluation.go`,
  `component/cluster.go`
- Database builder: `builder/store/database/space.go`
- Space deployment: when changing this flow, read
  `docs/agent/space-deployment.md` before editing.

## Merge Requests

Only when creating or updating a Merge Request:

1. Read `docs/agent/MR_CONTRACT.md`.
2. Follow its behavior-first MR structure.
3. Ensure the description matches the actual implementation and tests.
4. Include impact, root cause when applicable, solution details, and exact local
   test results.
