# Catalift: one repository, two applications (eng review D4).
#   server/    the Go API, worker and admin command (server/Makefile)
#   apps/web/  the React web app (apps/web/Makefile)
#   infra/     Terraform (infra/Makefile, its own gate)
# This Makefile only runs theirs. `make check` is the gate for both apps;
# `make server-<target>` and `make web-<target>` reach any other target.
SHELL := /bin/bash
.DEFAULT_GOAL := help
STATE := .bearing/state

.PHONY: help check check-file setup dev db migrate seed-demo test-integration

help: ## List targets
	@grep -E '^[a-zA-Z_%-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-18s %s\n", $$1, $$2}'

check: ## The gate: server, then web. CI runs each part in its own workflow.
	@mkdir -p $(STATE); rm -f $(STATE)/.check-passed
	@$(MAKE) --no-print-directory -C server check || { echo "check: server failed" >&2; exit 1; }
	@$(MAKE) --no-print-directory -C apps/web check || { echo "check: web failed" >&2; exit 1; }
	@touch $(STATE)/.check-passed; echo "check: server and web passed"

check-file: ## Lint one edited file, FILE=path from the root (the Bearing edit hook runs this)
	@[ -n "$(FILE)" ] || { echo "check-file: FILE is empty, nothing checked" >&2; exit 1; }; \
	case "$(FILE)" in \
	  server/*) $(MAKE) --no-print-directory -s -C server check-file FILE="$(patsubst server/%,%,$(FILE))";; \
	  apps/web/*) $(MAKE) --no-print-directory -s -C apps/web check-file FILE="$(patsubst apps/web/%,%,$(FILE))";; \
	  *) echo "check-file: no per-file check for $(FILE)";; \
	esac

setup: ## Install the server tools, the web dependencies and the git hooks
	$(MAKE) -C server setup
	$(MAKE) -C apps/web setup

dev: ## Run the API and worker (8080); run `make web-dev` in a second terminal for the web app (5173)
	$(MAKE) -C server dev

db: ## Start the local Postgres in Docker
	$(MAKE) -C server db

migrate: ## Apply the database migrations
	$(MAKE) -C server migrate

seed-demo: ## Create the local demo seller and reviewer accounts
	$(MAKE) -C server seed-demo

test-integration: ## Server tests against Postgres (needs make db and make migrate)
	$(MAKE) -C server test-integration

server-%: ## Any server target, e.g. make server-sqlc
	$(MAKE) -C server $*

web-%: ## Any web target, e.g. make web-dev
	$(MAKE) -C apps/web $*
