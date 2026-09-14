# Shared Code Smell Checks

This is the canonical structural checklist for both code generation and code
review in this repository. Apply it to every new or modified non-generated
function or method, including signature-only changes. Tests and lint do not
replace this check.

## Common Checks

- Inspect the signature, body, callers, and nearby tests. Check long or growing
  parameter lists, repeated input groups, same-typed positional arguments,
  redundant input sources, mixed responsibilities, duplicated rules/defaults,
  deep nesting, and coupling that prevents independent testing.
- Five or more business parameters (excluding receiver and context), roughly
  80-100+ body lines, or three or more distinct processing phases require a
  deliberate cohesion check. Adding another parameter to an already broad
  signature also triggers this check. These are signals, not hard limits;
  record a concrete reason when retaining such a shape.
- Look for concrete costs: callers supplying unrelated dependencies, input
  sources that can disagree, invariants depending on subtle statement ordering,
  duplicated error/compensation paths, and tests that must mock unrelated
  services to exercise one rule. Existing code and having only one caller are
  not sufficient reasons to keep extending an overloaded method.
- Prefer cohesive, layer-owned request/config structs and explicit context.
  Extract meaningful phases such as validation, argument construction, external
  calls, persistence, compensation, and projection. Do not hide unrelated inputs
  in a catch-all struct or split methods solely to meet a line limit.
- Do not pass a full request and its fields without a semantic reason. Preserve
  distinctions such as role-specific hardware versus request-wide hardware,
  along with authorization, error propagation, side-effect ordering, and
  compatibility.

## During Code Generation or Modification

Apply the checks before extending a function and again against the final diff.
When the change introduces or worsens a smell with concrete maintenance or
correctness costs, refactor the affected path during implementation.

Keep refactoring local to the task and affected callers. Do not expand into
unrelated cleanup, alter public contracts just to shorten a signature, or
hand-edit generated code. If necessary refactoring exceeds the authorized scope,
explain the limitation and remaining impact. Cover preserved behavior with
focused tests and follow the repository's validation and generation requirements.

## During Code Review

Complete a structural pass as well as the behavioral pass, even when behavioral
findings already exist. Apply the same common checks and report actionable
refactoring findings as `[P1]`, including maintainability-only defects. Stricter
API/correctness priorities still apply. Numeric thresholds or cosmetic
preferences alone do not justify findings; explain the concrete impact and
recommend responsibility-based refactoring seams.

Review mode does not authorize source edits, generation, or test execution.
Follow the applicable review workflow for CI evidence and local validation.

## Completion Check

This checks whether changed functions were inspected, not unit-test coverage.
It does not require local test coverage measurement; CI owns coverage checks.

Reconcile every new or modified function with the final diff. Keep concise
working notes per function: checked (with evidence), refactored, finding,
retained-with-reason, or unverified (with reason). Do not silently count
unverified items as passed. Report structural changes or findings and unresolved
limitations in the handoff; the full working inventory need not be published.
