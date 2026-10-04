@AGENTS.md

# CLAUDE.md

Claude Code specifics for this repository; the standard is AGENTS.md above.
Loaded into every session: keep this file under 40 lines.

```
Repository:   catalift, one repo (eng review D4): Go API and worker at the root,
              web app in apps/web (React, Vite), Terraform in infra/
Stack:        Go 1.26.8 + net/http + pgx + sqlc + goose + slog; React + TanStack; Terraform 1.9
Databases:    PostgreSQL
Entrypoint:   cmd/api/main.go (cmd/worker planned); apps/web/src/main.tsx
Run, test:    make dev, make test; gate: make check (root), and in apps/web, infra
Git host:     github, github.com/KarthikReddy8809/catalift (ADR-0011)
Tracker:      none (BEARING_TRACKER; none is valid)
Trunk:        main (no develop branch)
```

The Bearing plugin's hooks format and lint each edited file (`make
check-file` when the Makefile has it), add the task id from the branch,
refuse push, amend, rebase, merge, release and deploy commands, send you
back once when files you changed have not passed `make check`, and restore
the branch state and the user's last requests after compaction.
Permissions are in `.claude/settings.json`; `.claude/settings.local.json` may
add to them but must not loosen the deny list.

Plan mode for anything over three files or touching a schema, contract or
auth. On a third failed approach, stop and say so. Bound shell output with
`| tail -40`.

## Things the agent gets wrong in this repository

Add a line each time a mistake repeats; delete lines that stop applying.
