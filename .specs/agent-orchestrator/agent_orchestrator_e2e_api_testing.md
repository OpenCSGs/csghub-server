# Agent Orchestrator End-to-End API Testing Guide

This guide validates the public API contract without assuming a specific developer machine, cluster name, token, template ID, model, or resource ID. Supply values appropriate to the environment under test.

It covers:

- creating one immutable logical-agent definition;
- catalog and visibility updates;
- dedicated runtime allocation per actor and logical agent;
- pending-to-ready lifecycle and stable runtime reuse;
- admitted CSGClaw execution;
- visibility changes and durable cascade deletion;
- kind, isolation, no-PVC, credential, and failure assertions.

There is no logical-agent version API. Changing executable settings such as template, image, model, resource, environment, mode, or storage requires a new logical agent and content ID.

## 1. Prerequisites

- API, AIGateway, Temporal workers, Runner, PostgreSQL, and the target cluster are available.
- Logical writes and lifecycle flags are enabled only in the test environment.
- A public/accessible CSGClaw template and valid resource/model selection exist.
- The server-owned CSGClaw runtime profile has an image reference and readiness settings.
- `curl` and `jq` are installed.
- Use at least two signed-in users to verify actor isolation.

The server selects image, port, mounts, and integration configuration. Clients must not send them. For `read_only`, the final workload must have no PVC.

## 2. Environment variables

```sh
export API_BASE="https://hub.example/api/v1"
export AIGATEWAY_BASE="https://gateway.example/v1"
export PUBLISHER_TOKEN="replace-me"
export ACTOR_A_TOKEN="replace-me"
export ACTOR_B_TOKEN="replace-me"
export TEMPLATE_ID="70"
export RESOURCE_ID="3"
export MODEL_ID="replace-me"
export TEST_RUN_ID="$(date +%Y%m%d%H%M%S)"
export LOGICAL_CONTENT_ID="orchestrator-e2e-${TEST_RUN_ID}"
```

`API_BASE` includes `/api/v1`; `AIGATEWAY_BASE` includes its API version prefix. Use a fresh content ID for every run.

## 3. Create a logical agent

The product request contains no blueprint object:

```sh
CREATE_RESPONSE=$(curl --silent --show-error --fail-with-body \
  --request POST \
  --url "${API_BASE}/agent/instances" \
  --header "Authorization: Bearer ${PUBLISHER_TOKEN}" \
  --header "Content-Type: application/json" \
  --data-raw "$(jq -n \
    --arg name "Orchestrator E2E ${TEST_RUN_ID}" \
    --arg content_id "${LOGICAL_CONTENT_ID}" \
    --arg model "${MODEL_ID}" \
    --argjson template_id "${TEMPLATE_ID}" \
    --argjson resource_id "${RESOURCE_ID}" \
    '{
      name: $name,
      description: "Agent orchestrator E2E",
      type: "csgclaw",
      kind: "logical",
      content_id: $content_id,
      public: false,
      template_id: $template_id,
      metadata: {
        provision_request: {
          llm: {model: $model},
          resource_id: $resource_id,
          custom_ui: {welcome_text: "Welcome"},
          guest_usage_limit: {mode: "unlimited", max_count: 0}
        }
      }
    }')")

echo "${CREATE_RESPONSE}" | jq .
export LOGICAL_ID=$(echo "${CREATE_RESPONSE}" | jq -r '.data.id')
```

Assertions:

```sh
echo "${CREATE_RESPONSE}" | jq -e \
  --arg content_id "${LOGICAL_CONTENT_ID}" \
  '.data.kind == "logical" and
   .data.content_id == $content_id and
   .data.public == false and
   .data.mode == "read_only" and
   .data.storage_policy == "none" and
   (.data | has("generation") | not) and
   (.data | has("runtime_generation") | not) and
   (.data | has("runtime_instance_id") | not)'
```

Expected: HTTP `200`; no sandbox or Pod is created.

## 4. Verify executable immutability

Try to change model/resource/mode through the ordinary update API using the exact update envelope supported by the deployment. Any executable-field mutation must return `400`. A catalog-only update such as name, description, custom UI, or guest policy may succeed; catalog updates are last-write-wins and require no client version.

Also verify a client-supplied top-level `blueprint` on create returns `400`.

## 5. Make the logical agent public

```sh
VISIBILITY_RESPONSE=$(curl --silent --show-error --fail-with-body \
  --request PUT \
  --url "${API_BASE}/agent/instances/by-content-id/csgclaw/${LOGICAL_CONTENT_ID}" \
  --header "Authorization: Bearer ${PUBLISHER_TOKEN}" \
  --header "Content-Type: application/json" \
  --data-raw '{"public": true}')

echo "${VISIBILITY_RESPONSE}" | jq .
```

Assertions:

```sh
echo "${VISIBILITY_RESPONSE}" | jq -e \
  '.data.kind == "logical" and
   .data.public == true'
```

A visibility update issued while deletion is in progress must return `400`. The request cannot select another executable definition.

## 6. Use as actor A

```sh
export ACTOR_A_SESSION="${LOGICAL_CONTENT_ID}-actor-a"

use_actor_a() {
  curl --silent --show-error --include \
    --request POST \
    --url "${API_BASE}/agent/instances/${LOGICAL_ID}/use" \
    --header "Authorization: Bearer ${ACTOR_A_TOKEN}" \
    --header "Content-Type: application/json" \
    --data-raw "$(jq -n \
      --arg session_id "${ACTOR_A_SESSION}" \
      '{session_id: $session_id, session_name: "Actor A E2E"}')"
}

use_actor_a
```

First use may return:

- `202` with `status=provisioning` and `retry_after_seconds=5`; or
- `200` with `status=ready` if the runtime fence is already satisfied.

Do not poll by repeating `POST /use`. Poll the read-only status endpoint until ready:

```sh
status_actor_a() {
  curl --silent --show-error --fail-with-body \
    --request GET \
    --url "${API_BASE}/agent/instances/${LOGICAL_ID}/runtime-status" \
    --header "Authorization: Bearer ${ACTOR_A_TOKEN}"
}

while true; do
  STATUS_RESPONSE=$(status_actor_a)
  echo "${STATUS_RESPONSE}" | jq .
  STATUS=$(echo "${STATUS_RESPONSE}" | jq -r '.data.status')
  case "${STATUS}" in
    ready)
      break
      ;;
    provisioning)
      sleep 5
      ;;
    failed|deleting|released)
      echo "runtime reached terminal status: ${STATUS}" >&2
      exit 1
      ;;
    *)
      echo "unexpected runtime status: ${STATUS}" >&2
      exit 1
      ;;
  esac
done
```

The status response exposes only `status` and, while provisioning, `retry_after_seconds`. Status reads never create a second allocation or sandbox. Expected ready status: `.data.status == "ready"`, with no allocation ID, runtime instance ID, resource key, or sandbox content ID. Operator-side inspection should show one DNS-safe `aor-{logical-prefix}-{random}` allocation resource no longer than 63 characters.

Logical-agent chat may call the responses endpoint without Portal pre-warming. AIGateway waits up to 30 seconds for readiness, then either proxies the normal CSGClaw HTTP/SSE response or returns `503` with `Retry-After: 5`. Chat never returns `202`.

## 7. Verify actor isolation

Repeat `/use` with `ACTOR_B_TOKEN` and a different session:

```sh
export ACTOR_B_SESSION="${LOGICAL_CONTENT_ID}-actor-b"

ACTOR_B_USE=$(curl --silent --show-error --fail-with-body \
  --request POST \
  --url "${API_BASE}/agent/instances/${LOGICAL_ID}/use" \
  --header "Authorization: Bearer ${ACTOR_B_TOKEN}" \
  --header "Content-Type: application/json" \
  --data-raw "$(jq -n --arg session_id "${ACTOR_B_SESSION}" \
    '{session_id: $session_id, session_name: "Actor B E2E"}')")

echo "${ACTOR_B_USE}" | jq .
```

After actor B is ready, operator-side allocation and cluster inspection must show a different allocation/resource key from actor A. Repeating actor B use preserves actor B's original allocation.

Create a second logical agent with a different content ID and use it as actor A. It must create a distinct allocation/runtime; allocation reuse is scoped to logical agent plus actor.

## 8. Execute through AIGateway

Use the logical content ID as the catalog address and the same session ID:

```sh
curl --silent --show-error --include \
  --request POST \
  --url "${AIGATEWAY_BASE}/agent/csgclaw/agents/${LOGICAL_CONTENT_ID}/sessions/${ACTOR_A_SESSION}/responses" \
  --header "Authorization: Bearer ${ACTOR_A_TOKEN}" \
  --header "Content-Type: application/json" \
  --data-raw '{"input":"Hello","stream":false}'
```

Expected: HTTP `200` after admission and a CSGClaw response. `X-CSG-Agent-Execution-ID` is server-generated. Agent Server does not require or deduplicate an `Idempotency-Key`; every accepted request is a new execution attempt.

Start a second request **in the same session** while the first execution is still reserved/running/unknown. It must return `409` and must not forward to CSGClaw.

Concurrent requests in **different sessions** on the same actor runtime proceed in parallel up to the per-allocation cap (`orchestrator_max_concurrent_executions_per_allocation`, default 4). The first request beyond the cap returns `429` with `Retry-After: 5` and must not forward to CSGClaw.

To exercise suspension: leave the runtime idle past its sandbox timeout, confirm the allocation reports the suspended power state, then send a chat request. The request transparently resumes the same sandbox (no new allocation, unchanged runtime generation) and returns `200`; if the resume exceeds the wait timeout the response is `503` with `Retry-After: 5`.

## 9. Security and workload checks

For every allocated read-only runtime, inspect the final Pod including init containers, sidecars, and injected volumes:

- no PVC or generic ephemeral PVC exists;
- `agent_allocation_id` and `agent_runtime_generation` annotations match the allocation;
- the resource name matches the allocation's internal resource key and is absent from public API responses;
- readiness represents the CSGClaw application, not only Pod phase.

Verify ordinary sandbox get/start/stop/update/SSE and `/v1/sandboxes/{allocation_resource_key}` reject allocation-owned sandboxes with `409`. No response may contain:

- `CSGHUB_USER_TOKEN`;
- `CSGCLAW_ACCESS_TOKEN`;
- `CSGCLAW_LLM_API_KEY`;
- future platform-managed credentials.

Verify `/agent/instances` never lists allocation-owned sandboxes for any actor. The publisher cannot access actor A/B allocations merely by owning the logical agent, and actor A cannot access actor B's allocation or session.

## 10. Make the logical agent private

```sh
PRIVATE_RESPONSE=$(curl --silent --show-error --fail-with-body \
  --request PUT \
  --url "${API_BASE}/agent/instances/by-content-id/csgclaw/${LOGICAL_CONTENT_ID}" \
  --header "Authorization: Bearer ${PUBLISHER_TOKEN}" \
  --header "Content-Type: application/json" \
  --data-raw '{"public": false}')

echo "${PRIVATE_RESPONSE}" | jq .
```

Expected: new cross-user logical admission fails while owner use remains allowed. Existing actors remain usable through the authorized logical-agent route. No runtime is detached or mutated merely by changing visibility. Setting `public=true` again must not create another runtime for an actor that still has a live allocation.

## 11. Durable cascade deletion

```sh
curl --silent --show-error --include \
  --request DELETE \
  --url "${API_BASE}/agent/instances/${LOGICAL_ID}" \
  --header "Authorization: Bearer ${PUBLISHER_TOKEN}"
```

Deletion may return pending while Temporal removes actor runtimes. Poll the supported logical/deletion status until terminal. Verify:

- all allocation sandboxes are deleted;
- allocations are ended and orchestrated session bindings are removed as specified;
- capacity reservations are released exactly once;
- protected deploy credentials are cleared;
- retrying delete does not create duplicate operations or double-release capacity accounting;
- partial Kubernetes failure keeps deletion pending and is retried.

## 12. Negative matrix

| Case | Expected |
|---|---|
| Missing/empty/invalid logical `content_id` | `400` |
| Duplicate live `(type, content_id)` | `409` |
| Client supplies `blueprint` or protected credentials | `400` |
| Change model/resource/mode/storage/template on existing logical | `400` |
| Update a logical agent whose deletion has started | `400` |
| Delete a logical agent by content ID | `400` (logical cascade deletion is ID-addressed only) |
| Update another owner's logical agent | `403` or authorized-scope `404` |
| Non-owner uses a private logical agent | `403` or authorized-scope `404`; never provision |
| Same session UUID reused by another actor/logical agent | `409` |
| Allocation-owned sandbox get/start/stop/update/SSE | `409` |
| Second execution in the same session while occupied/unknown | `409` |
| Executions beyond the per-allocation concurrency cap | `429` with `Retry-After: 5` |
| Chat after idle suspension | transparent resume, `200`; `503` with `Retry-After: 5` on resume timeout |
| Ensure finds mismatched ownership/configuration labels | `409`/failed operation; never adopt |
| Creator token unavailable | provisioning fails closed; never use actor token |
| Logical slug equals an unrelated deploy name | never proxy unrelated deploy |

## 13. Anonymous share execution

Publish the logical agent, create a share link, and call the exact shared responses path with a persistent guest id:

```sh
GUEST_ID="$(uuidgen | tr '[:upper:]' '[:lower:]')"
curl --silent --show-error --fail-with-body \
  --request POST \
  --url "${AIGATEWAY_BASE}/shared/sandboxes/${SHARE_NAME}/api/v1/agents/${AGENT_NAME}/sessions/${SESSION_ID}/responses" \
  --header "Content-Type: application/json" \
  --header "X-CSG-Guest-ID: ${GUEST_ID}" \
  --data-raw '{"input":"hello","stream":false}'
```

Reuse the same `GUEST_ID` to reuse the same guest allocation. Any other shared path returns `404`. A missing or malformed guest id returns `400`. Guest-limit exhaustion returns `429`. A session collision returns `409`. Readiness timeout returns `503` with `Retry-After`. Clearing the stored guest id mints a new anonymous identity on the next request and can bypass the per-guest question limit and create another dedicated runtime.

## 14. Completion checklist

- [ ] Logical create stores one complete immutable definition and creates no sandbox.
- [ ] Executable changes require a new logical agent; catalog changes apply last-write-wins.
- [ ] Private logical agents are owner-usable; cross-user use requires `public=true`.
- [ ] Same actor plus logical agent reuses one allocation and sandbox.
- [ ] Different actors and different logical agents receive different allocations and sandboxes.
- [ ] Pending polling never creates an extra allocation or sandbox.
- [ ] Read-only workloads contain no PVC.
- [ ] Ordinary sandbox APIs cannot expose creator credentials or control allocation-owned sandboxes.
- [ ] AIGateway execution is admission-controlled and uses server-generated execution IDs.
- [ ] Kind dispatch prevents logical identifiers from reaching sandbox deploy lookup.
- [ ] Visibility changes preserve live bindings without creating new ones.
- [ ] Delete durably removes every allocated runtime and releases capacity accounting once.
- [ ] Failure/retry paths converge without leaked sandbox, session, operation, or capacity state.
