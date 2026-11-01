# Generation input contract (EE/SaaS)

Generation defaults to enabled for EE/SaaS when a Router server is configured.
Upgrade Router to support `lightgbm-rank-input/2` and matching service authentication
before upgrading Gateway, or explicitly set
`OPENCSG_AIGATEWAY_ROUTER_GENERATION_ENABLED=false` to retain compact v1.
Without a configured Router server, Auto routing remains disabled.

`OPENCSG_AIGATEWAY_ROUTER_CACHE_ENVIRONMENT` defaults to `default` and
`OPENCSG_AIGATEWAY_ROUTER_CACHE_NAMESPACE` defaults to `csghub`. Override these
when independent deployments share Router/storage. These scope names are retained
for compatibility; this change introduces no Gateway turn cache.

Gateway reuses the existing `STARHUB_SERVER_API_TOKEN` unless
`OPENCSG_AIGATEWAY_ROUTER_API_TOKEN_FILE` explicitly selects a dedicated credential.
Explicit missing or empty files fail initialization rather than falling back.
An empty resolved credential also fails. The publicly known default platform token
is rejected during initialization, even when supplied through the explicit file.
Override `STARHUB_SERVER_API_TOKEN` with a deployment-specific secret or, preferably,
configure `OPENCSG_AIGATEWAY_ROUTER_API_TOKEN_FILE` with a dedicated Router secret.
No credential value is included in the configuration error. The credential is sent as a Bearer token,
never copied from a user request. Disabled generation does not read the token file
or automatically forward the platform token.

Router must receive the same credential. Its existing file authentication can
consume the platform token through deployment automation; this MR does not modify
Router deployment scripts. Reusing the platform token extends its service trust
boundary to Router, and credential rotation requires both services to reload it.

## Input and identity

Gateway sends the existing compact projection plus `generation-config/1`:
protocol (`openai_chat`, `openai_responses`, `anthropic_messages`),
`value_stage=submitted`, explicitly supplied parameters and content control paths.
Missing fields remain absent. Explicit null, false and zero remain distinct.
No effective defaults, reasoning budget conversions or feature encoding are
performed in Gateway. Router and training own the shared feature encoder.

The extraction registry in `generation_input_ee.go` covers thinking, budget,
effort, output limits, sampling, tools, structured output and provider containers
such as `extra_body`. Unknown top-level fields are not automatically forwarded;
adding or renaming a field requires an explicit registry/contract compatibility
review. Do not place prompts or credentials in generation provider containers.
Full dialogue and tools are hashed locally into `source_input_identity`, not
copied into the routing request. Config identity uses a length-framed canonical
representation and SHA256. Frozen vectors preserve compatibility with Router.

Final serialized rank requests retain the Router's 2,000,000-byte limit.
Local overflow and remote HTTP 413 map to `routing_input_too_large` / HTTP 413
without withdrawing readiness. Generation JSON rejects duplicate keys, invalid
UTF-8, lone surrogate escapes, trailing JSON, nesting deeper than 64, numeric
coefficients longer than 4096 digits, and exponents outside +/-4096 (also after
fraction adjustment). These parser limits mirror the identity contract and
bound work; malformed submitted configuration preserves `model_not_found` / 404.
Remote 409/422 are input errors, also preserving 404 and readiness.

## Compatibility and rollback

Callers without an authenticated user/namespace scope, including bypass callers,
use compact v1 even with generation enabled and a turn header. No shared anonymous
scope is invented. Pinned upstream requests keep the existing direct path.
A v2 response must echo config/input identity and explicitly report
`generation_used`; false is valid when Router uses an old model. Missing/mismatched
binding is rejected, never treated as a valid v2 prediction. Rank response bodies
are bounded to 65,536 bytes before decoding.

This MR adds no turn snapshot, lease renewal, previous-call metadata or feedback.
Each rank request carries the submitted configuration and existing session/turn
headers. Router remains responsible for its own same-turn identity rules; input
conflicts are not silently retried with weaker binding.

Disable generation to return to compact v1 before rolling back a v2-capable
Router. This does not restore the pre-compact full-request contract. No model
training, model upload or deployment is performed by this change.

## Memory

Messages/Responses retain routing bytes only when the configured Auto model needs
generation, and release them before upstream streaming. CE retains no routing
snapshot through handler wiring. Canonical source hashing uses a bounded scratch
buffer instead of building another full canonical body. This limits additional
allocations, not total process RSS; full parsed request size still matters.

## Failure visibility and release gate

A missing/mismatched v2 binding is a dependency protocol failure (`BindingError`).
Gateway rejects it, returns routing unavailable (503), and withdraws Auto for all
callers sharing that Gateway router instance. This is process-local health state,
not a broadcast to other Gateway replicas. The rank-failure log includes
`error_kind=protocol_binding`; other rank errors use `error_kind=other` and retain
the existing error detail. Input rejection and size errors keep their existing
status/health handling.

Readiness alone does not prove v2 compatibility. A successful readiness probe can
restore Auto before a subsequent incompatible rank response withdraws it again.
Before enabling generation, every Router replica behind the load balancer must
support v2 and matching service authentication. A mixed v1/v2 rollout is not a
supported operating state. Upgrade Router first, then Gateway; if that order
cannot be guaranteed, explicitly disable generation until Router rollout finishes.
Gateway MRs !3240, !3241 (cache), and !3242 (feedback) are sequential code changes;
merging !3241 is not evidence that the Router service has been upgraded.

`GenerationDecoded` is reserved in this MR for request-local immutable reuse.
The subsequent feedback implementation in !3242 supplies it. Until supplied,
requests decode `GenerationRaw`; it is not an active production cache in !3240.

The following staging checks are release gates, not completed test results:

- [ ] Verify all Router replicas support v2 and have matching non-default credentials.
- [ ] Run Chat, Messages and Responses Auto requests through the real Gateway/LB/Router;
      verify submitted values and response identity binding, including `generation_used=false`.
- [ ] Verify wrong/missing bindings are rejected with 503, instance-local Auto withdrawal
      and a `protocol_binding` log; verify recovery after the dependency is corrected.
- [ ] Verify invalid credentials fail authentication and are distinguishable in logs;
      never use the public platform default in production.
- [ ] Verify generation=false retains compact v1 and perform the documented rollback order.
- [ ] Record environment, Gateway/Router revisions, evidence and result for each check.

Unit/component HTTP tests are automated evidence only; real staging validation
remains pending until this checklist is executed against the deployed services.
