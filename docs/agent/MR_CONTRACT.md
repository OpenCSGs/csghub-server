# Merge Request Review Contract

Every Merge Request (MR) MUST provide enough structured information for both human reviewers and AI review agents to verify that the implementation is correct.

The MR description is part of the implementation contract.

Its purpose is **not to summarize code changes**, but to describe:

* what system behaviors changed,
* why those behaviors changed,
* what behaviors must remain unchanged,
* what constraints the implementation must respect,
* and how the changed behaviors were verified.

## Behavior-first Principle

Merge Requests describe changes in **system behavior**, not changes in source files.

The primary review target is behavior.

Source code, tests, configuration, and documentation are evidence used to verify those behaviors.

> Source code is evidence; behavior is the review target.

MR descriptions SHOULD therefore be written so that a reviewer can understand the intended system changes before reading the diff.

---

# Required MR Structure

Every MR MUST contain the following sections:

1. Behavior Changes
2. Intent
3. Invariants
4. Non-goals
5. Design Constraints
6. Affected Components
7. Design Decisions
8. Risks
9. Verification
10. Breaking Changes

Sections MUST NOT be omitted simply because the change was implemented by an AI agent.

---

# 1. Behavior Changes

This is the most important section of the MR.

Describe every meaningful behavior that is introduced, removed, or modified.

Each behavior MUST:

* describe an observable system outcome,
* be independently understandable,
* be independently verifiable,
* avoid implementation-specific wording where possible.

Assign each behavior a stable identifier:

```text
B1
B2
B3
...
```

These identifiers SHOULD be referenced later in Verification and may also be referenced by review comments.

## External Behavior

Describe behavior observable by:

* users,
* API clients,
* administrators,
* integrations,
* external systems.

Example:

```text
B1. Organization members can access private repositories owned by their organization.

B2. Repository search applies the same authorization rules as repository listing.

B3. Anonymous users cannot discover private repositories through search.
```

## Internal Behavior

Describe important system behavior that may not be directly user-visible but is relevant to correctness.

Examples include:

* cache invalidation,
* authorization evaluation,
* consistency behavior,
* retry behavior,
* failover behavior,
* transaction boundaries,
* scheduling behavior,
* state transitions,
* concurrency semantics,
* persistence behavior.

Example:

```text
B4. Organization membership changes invalidate the authorization scope cache.

B5. OpenFGA is queried only when the authorization scope requires refresh.

B6. Cursor pagination preserves authorization filtering across subsequent pages.
```

## Avoid

Do NOT describe behaviors as source-code changes.

Avoid:

```text
- Updated auth.go.
- Refactored repository service.
- Added filtering logic.
- Modified cache implementation.
```

Prefer:

```text
B1. Repository listing excludes repositories outside the user's authorization scope.

B2. Authorization cache entries are invalidated when organization membership changes.
```

---

# 2. Intent

Explain why the behavior changes are necessary.

Describe:

* the problem with the current behavior,
* the user, product, operational, or architectural motivation,
* why this MR is needed now.

Do NOT repeat the implementation.

Do NOT merely restate the Behavior Changes section.

Avoid:

```text
Implement organization permission filtering.
```

Prefer:

```text
Repository listing currently applies authorization after query execution, which can return incomplete pages and inconsistent search results.

This change moves authorization scope into the query path so listing and search use the same visibility semantics.
```

---

# 3. Invariants

List behaviors that MUST remain unchanged.

Invariants are part of the review contract and MUST be actively verified.

Examples:

```text
I1. Repository owners always retain access to their own repositories.

I2. Administrator authorization bypass behavior remains unchanged.

I3. Existing API response schemas remain backward compatible.

I4. Cursor pagination semantics remain unchanged.

I5. Public repositories remain accessible without authentication.
```

Review agents SHOULD explicitly check whether the implementation violates any listed invariant.

---

# 4. Non-goals

Explicitly define what this MR intentionally does not address.

Non-goals prevent reviewers and review agents from interpreting deliberate scope boundaries as missing implementation.

Example:

```text
This MR does not:

- change the OpenFGA authorization model,
- introduce file-level authorization,
- redesign organization membership management,
- optimize unrelated repository queries.
```

Non-goals MUST NOT be used to hide issues introduced directly by the implementation.

---

# 5. Design Constraints

Describe important constraints the implementation MUST respect.

Examples include:

## Compatibility

```text
- Existing API contracts must remain backward compatible.
- Existing database records must remain readable.
```

## Performance

```text
- Repository listing must not perform one authorization request per repository.
- The change must not introduce N+1 database queries.
```

## Scalability

```text
- The design must support large organizations without materializing user-resource permission relationships.
```

## Security

```text
- Authorization must be applied before resources are returned.
- Search and listing must use identical visibility semantics.
```

## Operational

```text
- The change must not require service downtime.
- Existing deployments must remain upgradeable without manual intervention.
```

Review agents SHOULD treat violations of explicit constraints as review findings.

---

# 6. Affected Components

List the major subsystems affected by the behavior changes.

Describe logical components rather than individual files.

Examples:

```text
Affected components:

- Repository Service
- Search Service
- Authorization Layer
- OpenFGA Integration
- Authorization Cache
- Repository API
```

Do NOT use this section as a file list.

Avoid:

```text
- repository.go
- auth.go
- handler.go
```

---

# 7. Design Decisions

Explain important implementation decisions when multiple reasonable approaches exist.

Focus on decisions that a reviewer may need to evaluate.

Examples:

* why a specific architecture was chosen,
* why a cache is introduced,
* why a query-time approach was selected,
* why one authorization strategy was preferred,
* why a new abstraction or dependency was introduced,
* why an existing approach was rejected.

Include meaningful trade-offs where appropriate.

Example:

```text
Authorization scope is resolved before repository querying instead of filtering results after retrieval.

This avoids incomplete pages and keeps pagination semantics consistent.

The trade-off is that authorization scope refresh becomes part of the request path when the cache is stale.
```

Do NOT explain trivial implementation details that are already obvious from the diff.

---

# 8. Risks

Identify areas where regressions or incorrect behavior are most likely.

Examples:

```text
Risks:

- stale authorization scope after membership changes,
- differences between search and listing behavior,
- pagination edge cases,
- incorrect fallback behavior when OpenFGA is unavailable,
- performance degradation for users belonging to many organizations.
```

Review agents SHOULD prioritize these areas during review.

---

# 9. Verification

Verification MUST be mapped back to declared behaviors and invariants.

Avoid generic statements such as:

```text
Tests passed.
```

Prefer explicit behavior verification.

Example:

```text
B1 ✓ Integration test
Organization members can access organization repositories.

B2 ✓ Integration test
Search and listing return resources using the same authorization scope.

B3 ✓ Integration test
Anonymous users cannot discover private repositories.

B4 ✓ Unit test
Membership changes invalidate cached authorization scope.

B5 ✓ Unit test
Cached authorization scope avoids unnecessary OpenFGA requests.

B6 ✓ Integration test
Authorization filtering remains correct across cursor pagination.

I1 ✓ Existing repository-owner test suite

I2 ✓ Administrator authorization regression tests

I3 ✓ API compatibility tests
```

Verification methods may include:

* unit tests,
* integration tests,
* end-to-end tests,
* regression tests,
* benchmarks,
* manual verification,
* load tests,
* compatibility tests,
* static analysis.

If a behavior cannot be verified automatically, state how it was manually validated.

If a declared behavior has no verification, explicitly state that.

Example:

```text
B7 ⚠ Not covered by automated tests.
Validated manually against a staging environment.
```

---

# 10. Breaking Changes

Describe any incompatible change in:

* APIs,
* stored data,
* configuration,
* deployment,
* authorization semantics,
* operational behavior,
* integrations.

Example:

```text
Breaking Changes:

- API now returns HTTP 403 instead of HTTP 404 for unauthorized repository access.
```

If there are none, explicitly state:

```text
Breaking Changes: None
```

---

# MR Description Template

Agents creating an MR SHOULD use the following structure.

```markdown
## Behavior Changes

### External Behavior

B1. ...

B2. ...

### Internal Behavior

B3. ...

B4. ...

## Intent

...

## Invariants

I1. ...

I2. ...

## Non-goals

- ...
- ...

## Design Constraints

- ...
- ...

## Affected Components

- ...
- ...

## Design Decisions

- ...
- ...

## Risks

- ...
- ...

## Verification

B1 ✓ ...
B2 ✓ ...
B3 ✓ ...

I1 ✓ ...
I2 ✓ ...

## Breaking Changes

None
```

---

# MR Authoring Rules for AI Agents

When an AI agent creates an MR, it MUST derive the MR description from the actual implementation and task context.

The agent MUST NOT:

* invent behaviors that were not implemented,
* claim verification that was not performed,
* claim tests passed when they were not executed,
* hide uncertainty,
* copy generic boilerplate without adapting it to the change,
* describe only source files,
* describe only implementation mechanics.

If information is uncertain, state that explicitly.

Example:

```text
B4 ⚠ Expected to preserve retry semantics, but this was not verified by an integration test.
```

---

# AI Review Contract

The MR description is a specification to be verified, not a trusted statement of fact.

Review agents MUST compare the declared contract against:

* implementation,
* tests,
* configuration,
* schemas,
* migrations,
* affected interfaces,
* surrounding repository behavior.

The review agent SHOULD verify:

## Behavior Verification

* Is every declared behavior actually implemented?
* Are declared behaviors implemented consistently across all affected paths?
* Are any important behavior changes missing from the MR description?
* Does the implementation introduce unintended behavior?

## Invariant Verification

* Is every declared invariant preserved?
* Could the implementation regress existing behavior outside the primary change path?

## Scope Verification

* Does the implementation stay within the stated scope?
* Does it unintentionally modify a listed non-goal?

## Constraint Verification

* Are security, performance, compatibility, and scalability constraints respected?

## Verification Quality

* Does each important behavior have sufficient test coverage?
* Do the tests verify behavior rather than merely implementation details?
* Are important edge cases missing?

## Description / Implementation Consistency

If the MR description and implementation disagree, the discrepancy MUST be reported.

Examples:

```text
B3 claims search and listing use identical authorization rules, but the search endpoint bypasses organization scope filtering.
```

```text
I2 states administrator behavior is unchanged, but the new middleware now applies authorization checks before the administrator bypass.
```

```text
The MR declares no breaking changes, but the API response changed from 404 to 403.
```

A mismatch between the MR contract and implementation is itself a valid review finding.

---

# Review Priority

Review agents SHOULD prioritize findings in the following order:

1. Incorrect or missing declared behavior
2. Invariant violations
3. Security and authorization issues
4. Data integrity and correctness
5. Concurrency and consistency issues
6. Breaking compatibility changes
7. Architectural constraint violations
8. Performance and scalability regressions
9. Missing or insufficient verification
10. Local code quality issues

Code-style observations SHOULD NOT distract from correctness, behavior, or architectural findings.

---

# Core Principle

A high-quality MR allows a reviewer to establish the intended mental model of the change before reading the code.

The expected review flow is:

```text
Intent
    ↓
Behavior Contract
    ↓
Invariants / Constraints
    ↓
Implementation
    ↓
Tests
    ↓
Verification
```

The goal is not merely to determine whether the code looks correct.

The goal is to determine whether the implementation produces the intended behavior while preserving the required properties of the system.
