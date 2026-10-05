# Catalift

Turns a seller's product CSV and photos into compliant, reviewer-approved listings for each sales channel. One repository (eng review D4) on GitHub (ADR-0011): `github.com/KarthikReddy8809/catalift`.

## What is here today

| Part | Path | Stack (pin and source) | Gate |
| --- | --- | --- | --- |
| Server | `server/` (`cmd/api`, `cmd/worker`, `cmd/admin`, `internal/`, `db/`, `config/channels`) | Go 1.26.8 (server/go.mod; chosen 2026-10-04 over the local 1.25 because sqlc, goose and govulncheck need 1.26), net/http, pgx, sqlc, goose | `make check` in `server` |
| Web app | `apps/web` | React, TypeScript, Vite, TanStack, shadcn/ui, Node 24, pnpm 9.15.9 | `make check` in `apps/web` |
| Infrastructure | `infra` | Terraform 1.9.8, one environment `prod` | `make check` in `infra` (needs tflint, trivy, checkov, shellcheck) |
| Design | `docs/`, `api/openapi.yaml`, `TODOS.md` | PRD, backlog, HLD v3, ADR-0001 to ADR-0011, data model, API spec, threat model | per-skill gates |

The root `Makefile` runs the two applications' Makefiles: `make check` runs the server gate, then the web gate. `make server-<target>` and `make web-<target>` reach any other target.

What runs today: sign-in, CSV and photo upload, attribute detection, listing generation, the rules engine, the review grid with edits and regeneration, bulk approval, the CSV export and the AI cost ledger. Without `OPENROUTER_API_KEY` a free local stand-in answers every AI call.

## Run it locally

```
cp .env.example .env
GOTOOLCHAIN=go1.26.8 make setup
make db && make migrate && make seed-demo
make dev
```

Then `make web-dev` in a second terminal and open http://localhost:5173. `.env` stays at the repository root; the server reads it from there. Sample data to try the flow: `docs/samples/autumn-launch/`.

Without Go 1.26.8 installed, `GOTOOLCHAIN=go1.26.8` lets the go command fetch it.

## Planned (not built)

- The VM, data disk, buckets, alerts and WIF bootstrap in `infra` (HLD v3 sections 3, 10, 12).
- A deploy workflow that pulls images onto the VM and restarts compose (no deploy job exists yet).

## Owners

`@KarthikReddy8809` (CODEOWNERS).
