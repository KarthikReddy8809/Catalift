# ADR-0004: Use Go for the backend, rebuilding the AI gateway and job runner in Go

- Status: Accepted
- Date: 2026-10-01
- Task: none
- Deciders: Karthik Reddy
- Area: backend language
- Reversibility: awkward: every service, the rules engine and the gateway are written in it; changing language later is a rewrite

## Context

- From the request: "for the backend i have decided to use the golang".
- The PRD constraints say Catalift is built on Bearing's `brg_llm_gateway`, `brg_jobs` and `brg_design_variants` (docs/product/PRD.md, Constraints; Q-017). None of them is present on this machine, and their snake_case names suggest Python packages, so they cannot be imported into a Go service as they are.
- The backend's work is CSV upload, a code-based rules engine, CRUD for the review grid, CSV export, background jobs and calls to OpenRouter over HTTP (US-00-001 to US-00-012). None of it needs a Python-only library.
- The gateway's required behaviour is small and fixed by the backlog: record tokens and cost per call (US-00-011), refuse calls once spend passes USD 8 (US-00-012), and replay recorded responses in tests (PRD Constraints).

## What else was considered

| Option | Why not | Would suit |
| --- | --- | --- |
| Go (chosen) | the gateway, job runner and test replay are rebuilt rather than reused, which goes against the brief's "built on existing components" | services and APIs with a small, well-defined AI surface over HTTP |
| Python (FastAPI) | slower than Go, and dependency management is heavier for the rest of the backend | reusing the `brg_` components unchanged, or heavier AI or data work |
| Go API plus a Python AI worker | two languages, two images and a job contract to keep in step | the `brg_` components are Python and must be reused as they are |

## Decision

We will write the backend in Go and implement the AI gateway (per-call token and cost record, USD 8 spend block, recorded-response replay for tests) and the background job runner in Go, because the rest of the backend suits Go, the gateway's behaviour is small and fully specified by US-00-011 and US-00-012, and one language keeps the VM deploy (ADR-0002) to one service image.

## Consequences

- The brief's dependency on `brg_llm_gateway` and `brg_jobs` becomes "follow their design, written in Go". Q-017 in docs/product/questions.md should be updated to say so.
- `brg_design_variants` is not replaced here; what it does for Catalift is still unknown (Q-017).
- OpenRouter is called over plain HTTP from Go; image input for detection is sent as base64 or a URL in the request.
- Still to decide: data access (sqlc with pgx is the standard default), the job queue (a Postgres-backed queue is the standard default, since Postgres is already there), and the API style (REST with OpenAPI is the default for a single-page app).
- Revisit if the `brg_` components turn out to be Go, so they can be reused, or if later AI work (evaluation, fine-tuning) needs Python libraries; then add a Python worker rather than switch languages.

## Commits us to

Go (current stable release), net/http or a router chosen at build time, OpenRouter HTTP API, `claude-haiku-4-5`
