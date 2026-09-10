# Space Deployment Agent Guide

This is an agent-only implementation guide for changes to Space image builds and
deployments. Read it before editing any part of the flow, and verify the complete
path rather than treating one handler or component in isolation.

## End-to-End Flow

1. `builder/deploy/deployer.go` creates build and deployment tasks in the
   database and starts the Temporal workflow.
2. `api/workflow/activity/deploy_activity.go` implements Temporal activities
   that call the Runner API.
3. `runner/handler/imagebuilder.go` accepts image-build requests and delegates
   them to the image-builder component.
4. `runner/component/imagebuilder.go` creates and monitors the image build as an
   Argo Workflow in Kubernetes.
5. `runner/handler/service.go` accepts service deployment requests and delegates
   them to the service component.
6. `runner/component/service.go` creates or updates the deployed service through
   Knative.
7. `docker/spaces/builder/Dockerfile*` defines the images used to build Spaces.

## Change Checklist

- Trace request and task identifiers across API, Temporal, Runner, and database
  boundaries.
- Preserve retry and idempotency behavior. Temporal activities and Runner calls
  may execute more than once.
- Check status transitions for build, deployment, failure, cancellation, and
  timeout paths.
- Keep API and Runner request/response contracts compatible. Update both sides
  and their tests when a contract must change.
- Verify context cancellation and deadlines on outbound calls.
- Avoid placing orchestration or deployment business rules in handlers.
- When Dockerfiles change, check every matching builder variant and confirm that
  build arguments, runtime architecture, and required tools stay aligned.
- Add or update focused tests in each affected layer, then run tests for every
  affected edition as described in the repository `AGENTS.md`.

## Related References

- `component/space.go` contains Space business behavior.
- `api/handler/space.go` exposes Space API operations.
- `component/cluster.go` contains cluster-related business behavior.
- `runner/router/api.go` registers Runner endpoints.
