# CSGClaw Custom UI Space Specification

## Purpose and v1 boundary

A CSGClaw agent can be bound to a separately deployed Space as its custom UI.
The agent's custom domain routes to that Space, while the Space backend
invokes the agent through AIGateway's shared-sandbox proxy. The Space is
deployed before the agent is created and is selected during agent creation.

The bound Space may be public or private. A private bound Space keeps its repo
content and its own endpoint access-controlled; its deployed page becomes
anonymously reachable only through the bound agent's custom domain, authorized
by the binding itself. Binding a private Space requires write permission on
it, because binding is a publish action.

Custom UI binding does not restrict agent visibility at creation. In v1, runtime
custom-domain routing and anonymous shared-sandbox invocation are supported for
public agents; private-agent authenticated invocation remains deferred. A
private bound agent may be promoted to public, which creates its automatic
share. Public custom-UI agents cannot be made private because the automatic
public share cannot currently be revoked. No browser configuration endpoint or
second agent-selection step is used: the custom domain itself identifies the
agent.

## Relationship and eligibility

```text
Agent custom domain
  -> CSGClaw agent instance
  -> metadata.provision_request.custom_ui_space_id
  -> Agent UI Space (task / agent-ui tag)
  -> latest Running Space deployment
```

The Space ID is the binding source of truth. The built-in tag is for product
discovery and creation-time validation, not runtime routing. The selection API
returns readable tagged Spaces with a Running deployment — public ones for any
user, plus explicitly granted private ones; it does not take an
`agent_public` query parameter. Creation repeats validation and requires read
access for a public Space or write access for a private Space; the agent may
be either public or private.

## Main flow

### 1. Prepare and select a UI Space

Deploy the Space before creating the agent and assign the existing tag:

```text
scope: space
category: task
name: agent-ui
```

The portal can also create such a Space directly via `POST
/agent/instances/ui-spaces`: the request names the target `namespace`
(required) plus an optional `name`, `nickname`, and `description`; when `name`
is omitted, the server generates a unique repo name
(`agent-ui-<random 8-char suffix>`). The server seeds the pinned
framework bundle, auto-selects a free compute resource, and attaches the tag.
The response returns the new `space_id` and full `path` (see
[custom-ui-space-api.md](custom-ui-space-api.md)).

The authenticated frontend obtains candidates from `GET
/agent/instances/ui-options`, with the usual `per` and `page` pagination query
parameters plus an optional `namespace` filter that restricts the result to
Spaces under a given user or org namespace. The API returns the standard
response envelope: options in `data` and the unpaginated count in top-level
`total`. Each option includes the Space's
public `endpoint` (in the same host/path form used by the Space API):

```json
{
  "msg": "OK",
  "data": [
    {
      "space_id": 123,
      "path": "owner/my-agent-ui",
      "name": "my-agent-ui",
      "private": false,
      "sdk": "gradio",
      "status": "Running",
      "endpoint": "my-ui-space.spaces.opencsg.com"
    }
  ],
  "total": 1
}
```

The options are readable Spaces tagged `task / agent-ui` — public Spaces for
any user, plus granted private ones — regardless of deploy state. Each option
carries an aggregated `status`: `Running` (with a live `endpoint`) or
`Unavailable` (building, failed, stopped, or never deployed; no endpoint). The
frontend uses `space_id` as the binding value; only `Running` options bind
successfully. The option's `private` flag reports repo visibility: a private
bound Space is reachable only through the agent's custom domain, never via its
own `endpoint`.

Agent creation includes the selected ID in
`metadata.provision_request.custom_ui_space_id`. The optional `subdomain` field
selects the agent's custom hostname and is honored only when a custom UI Space is
bound:

```json
{
  "type": "csgclaw",
  "name": "my-agent",
  "public": true,
  "metadata": {
    "provision_request": {
      "custom_ui_space_id": 123,
      "subdomain": "my-agent"
    }
  }
}
```

The server validates the Space type, tag, latest Running deployment, and the
creator's access: read permission for a public Space, write permission for a
private Space. It stores the bound Space ID in the provision request. The subdomain must be a valid lowercase DNS label (1–63
letters, digits, or hyphens; it must start and end with a letter or digit) and
must not conflict with an active deployment name. It becomes both the sandbox
service name and the agent `content_id`; without a custom UI binding, CSGClaw
uses its existing generated sandbox name and ignores custom-UI subdomain
behavior. The created agent endpoint is stored in
`metadata.provision_response.endpoint`, for example
`https://my-agent.spaces.opencsg.com`.

If `subdomain` is omitted, the server generates the existing CSGClaw sandbox
name; the resulting `content_id` is used as the hostname label.

### 2. Route browser traffic and invoke the agent

```text
 Browser (anonymous)           RProxy                  UI Space backend          AIGateway             Agent sandbox
        |                         |                           |                       |                       |
        | GET https://my-agent.space.../                         |                       |                       |
        |------------------------>|                           |                       |                       |
        |                         | resolve host -> public     |                       |                       |
        |                         | CSGClaw agent (content_id)  |                       |                       |
        |                         | resolve bound UI Space +   |                       |                       |
        |                         | latest Running deployment  |                       |                       |
        |                         | look up generated share    |                       |                       |
        |                         | name s-a-42                |                       |                       |
        |                         | strip browser credentials |                       |                       |
        |                         | inject URL, mode, kind, name |                       |                       |
        |                         |-------------------------->|                       |                       |
        |                         |  UI page/API request +     |                       |                       |
        |                         |  URL + auth mode + kind + name |                       |                       |
        |                         |                           |                       |                       |
        |                         |                           | server-side action:   |                       |
        |                         |                           | read context; use     |                       |
        |                         |                           | agent name in API path|                       |
        |                         |                           |---------------------->|                       |
        |                         |                           | GET {base URL}/...    | resolve share and    |
        |                         |                           |                       | proxy request ------>|
        |                         |                           |<----------------------|<----------------------|
        |<------------------------|<--------------------------|                       |                       |
        |           UI response   |                           |                       |                       |
```

The flow applies to all requests routed through the custom domain, including
static assets and UI API calls. RProxy resolves the current Space deployment
for routing; it does not persist a service name, so redeployments do not require
rebinding. RProxy resolves a reused service name through a dedicated newest-row
lookup, including Deleted rows, so a reclaimed name follows its current
lifecycle. This lookup is specific to RProxy; the shared deploy-by-service-name
query and webhook status behavior remain unchanged. The Space server-side
code—not browser JavaScript—uses the injected context to call AIGateway's
anonymous shared-sandbox route:

```text
GET {agent_base_url}/{path}
```

The exact method, path, and payload are determined by the existing
AIGateway-compatible agent invocation contract. The Space must not infer an
agent from its own service name.

## Trusted proxy context

For each custom-domain request, RProxy resolves the agent from the host and
looks up its automatically generated share using
`types.CSGClawAutomaticShareName(agent.ID)`. It verifies the share belongs to
that agent, then constructs the shared-sandbox base URL from the configured
AIGateway base URL and injects it into the request forwarded to the selected
Space:

```text
X-CSGHub-Agent-Base-URL: https://<aigateway-domain>/v1/shared/sandboxes/s-a-42
```

RProxy removes/overwrites any incoming copy of this header and strips browser
authorization, cookies, and platform bot credential headers before forwarding
to the public Space. It also injects the auth mode and the agent instance kind:

```text
X-CSGHub-Agent-Auth-Mode: anonymous
X-CSGHub-Agent-Kind: logical
```

`X-CSGHub-Agent-Kind` is `logical` or `runtime`. The Space sends `X-CSG-Guest-ID`
only when the kind is `logical`.

The Space backend appends the Agent API path to the injected URL and calls it
directly; the Space does not need AIGateway URL configuration in its
environment. In `anonymous` mode, the Space must not forward a user token. The
custom UI frontend or SDK generates one UUIDv4, persists it as
`opencsg_guest_id` in local storage, and sends it as `X-CSG-Guest-ID` on every
shared logical-agent chat request. RProxy forwards that header to the Space so
the Space can copy it onto its AIGateway call; ordinary Space proxying strips
the header because it is platform-reserved. Deleting `opencsg_guest_id` creates
a new anonymous identity. A future `user-token` mode would tell the Space to
attach the authenticated user's token. Private-agent custom UI and token
forwarding are not supported in v1, but that runtime limitation does not
prevent creating a private agent with a custom UI binding. RProxy
removes/overwrites client-provided copies of the trusted context headers, so
browser input cannot select the URL, auth mode, agent kind, or agent name.

The CSGClaw API path also requires the manifest agent name, which is different
from the instance content ID and display name. RProxy reads
`metadata.template_metadata.agent_file.name`, validates it, and overwrites
`X-CSG-Agent-Name` with that value. The Space uses this trusted name when
building the path under the shared-sandbox base URL, for example
`/api/v1/agents/{agent_name}/sessions/{session_id}/responses`.

These values are trusted routing/authentication instructions, not proof of user
identity or authorization. The Space must not use them to access private agents
or other privileged resources. The v1 share is public by design. RProxy also
overwrites client-provided `X-CSG-Agent-Name`; the Space must not use a
browser-provided agent name.

The browser does not call `/__csgclaw/config`, select an agent, or receive the
share name as a frontend configuration response. Owner credentials, platform
API keys, and internal service addresses are never exposed to browser code.

## Authentication and visibility

- Agents of either visibility may be created with a custom UI binding.
- The selected UI Space must be tagged `task / agent-ui` and have a latest
  Running deployment. A public Space requires the creator's read access; a
  private Space additionally requires write access, because binding publishes
  its deployed page through the agent's custom domain.
- A private bound Space keeps its repo and its own endpoint access-controlled;
  RProxy's custom-UI path authorizes anonymous traffic to its deployment
  through the agent's custom domain using the binding alone, resolved per
  request. Unbinding or deleting the Space cuts that access immediately. For a
  private bound Space, RProxy also re-checks on every request that the agent
  owner still has write permission on the Space; losing that permission (for
  example org membership revoked after binding) fails the request closed.
- Anonymous users may visit the agent custom domain and invoke the agent using
  its generated public share through AIGateway.
- Private-agent custom-domain routing and delegated user-token forwarding are
  out of scope for v1; RProxy denies a private bound agent rather than proxying
  it. A private binding can be promoted to public and receives its automatic
  share; demoting a public custom-UI agent is rejected until share revocation is
  supported.
- The public shared-sandbox route must retain its existing share-level
  authorization and rate/usage controls.

## Lifecycle and failures

- Removing `agent-ui` hides a Space from new selections but does not invalidate
  an existing binding.
- A missing binding or deleted Space cannot be routed; a stopped/non-running
  deployment is unavailable.
- Redeploying a Space changes the runtime target without changing the agent
  binding or endpoint.
- Missing or mismatched automatic share data is a server error; RProxy must not
  accept a browser-provided share name as fallback.
- Deleting the agent removes its custom-domain route and automatic share.

## Out of scope

- Private-agent UI and authentication delegation.
- Building or deploying the custom UI frontend itself.
- Changes to AIGateway's shared-sandbox proxy behavior.
- A browser-facing agent configuration endpoint or agent-selection UI.
