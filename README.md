# Catalift

Turns a seller's product CSV and photos into compliant, reviewer-approved listings for each sales channel. One repository (eng review D4) on GitHub (ADR-0011): `github.com/KarthikReddy8809/catalift`.

## What is here today

| Part | Path | Stack (pin and source) | Gate |
| --- | --- | --- | --- |
| API | `cmd/api`, `internal/`, `db/` | Go 1.26.8 (go.mod; chosen 2026-10-04 over the local 1.25 because sqlc, goose and govulncheck need 1.26), net/http, pgx, sqlc, goose | `make check` at the root |
| Web app | `apps/web` | React, TypeScript, Vite, TanStack, shadcn/ui, Node 24, pnpm 9.15.9 | `make check` in `apps/web` |
| Infrastructure | `infra` | Terraform 1.9.8, one environment `prod` | `make check` in `infra` (needs tflint, trivy, checkov, shellcheck) |
| Design | `docs/`, `api/openapi.yaml`, `TODOS.md` | PRD, backlog, HLD v3, ADR-0001 to ADR-0011, data model, API spec, threat model | per-skill gates |

What the code does today is the kit skeleton only: the API serves health and readiness routes and connects to Postgres; the web app renders a health page; infra declares a network and a service account module. None of the Catalift features exist yet.

## Run the API locally

```
cp .env.example .env
GOTOOLCHAIN=go1.26.8 make setup
make db && make migrate
make dev
```

Without Go 1.26.8 installed, `GOTOOLCHAIN=go1.26.8` lets the go command fetch it.

## Planned (not built)

- `cmd/worker`: the job worker and AI gateway (ADR-0005, HLD v3 section 3).
- The domain areas auth, catalogue, listings, export, cost (HLD v3, tenet 1).
- Migration 1 from `docs/design/schema.sql`, replacing the kit's `db/migrations/00001_init.sql` probe.
- The VM, data disk, buckets, alerts and WIF bootstrap in `infra` (HLD v3 sections 3, 10, 12).
- A deploy workflow that pulls images onto the VM and restarts compose (no deploy job exists yet).

## Owners

`@KarthikReddy8809` (CODEOWNERS).
