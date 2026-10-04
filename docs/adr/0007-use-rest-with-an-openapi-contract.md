# ADR-0007: Use REST with an OpenAPI contract between the web app and the API

- Status: Accepted
- Date: 2026-10-01
- Task: none
- Deciders: Karthik Reddy
- Area: api style
- Reversibility: awkward: every screen calls the API through the generated client; changing style means rewriting the client layer and the handlers

## Context

- One React single-page app (ADR-0003) talks to one Go API (ADR-0004); no mobile app or outside consumer is planned (PRD non-goals: no marketplace publishing).
- The frontend uses TanStack Query through one axios client (ADR-0003); typed client code from a contract avoids hand-written request and response types.
- The go-api kit stack (net/http) defaults to REST with OpenAPI.

## What else was considered

| Option | Why not | Would suit |
| --- | --- | --- |
| REST with OpenAPI (chosen) | the spec must be kept in step with the handlers; CI checks it | one or a few clients over HTTP, generated types |
| REST with no contract | types drift between Go and TypeScript; nothing for reviews and tests to point at | a throwaway prototype |
| GraphQL | a schema server and resolvers for one client with simple screens | many clients needing different shapes of the same graph |
| gRPC (Connect) | browser support needs Connect or a proxy; outside the standard stack | service-to-service calls between several backends |

## Decision

We will expose a REST API described in `api/openapi.yaml`, versioned under `/v1`, and generate the TypeScript types and axios client from it, because it is the catalogue and kit default, it gives the frontend typed calls for TanStack Query, and one contract file is what the HLD and tests point at.

## Consequences

- `api/openapi.yaml` is the source of truth; a CI check fails when the handlers and the spec disagree.
- Long-running work (detection, generation) is started by a POST that returns at once, and progress is read by polling (ADR-0005 runs the work).
- File uploads (CSV, images) use multipart form requests described in the spec.
- Revisit if a second backend service calls this one (then gRPC or Connect inside), or if a public API for sellers is planned (then a versioning and deprecation policy).

## Commits us to

OpenAPI 3.1, an OpenAPI-to-TypeScript generator chosen in the LLD, axios (ADR-0003)
