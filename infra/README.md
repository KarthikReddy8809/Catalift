# Catalift infra

The infrastructure part of the Catalift repository (ADR-0008). `make help` lists every command; `make check` is
the gate. CI plans and posts the plan; a person applies. Nothing here
applies on its own.

## Environment

One environment, `prod`, in `envs/prod`, applied from `main` by the manual
`infra` workflow after approval in the `prod-apply` GitHub environment
(ADR-0011). Catalift is a demo with at most about 10 users a minute, so there
is no dev or qa environment: changes are checked by `make check` and
`terraform plan` on the pull request, and replay mode (HLD v3 section 12)
stands in for a test environment.

## State backend

`envs/<env>/backend.tf` carries a `__BACKEND__` block with the GCS backend
active and the S3 backend beside it, commented. The `cloud` key of the
`tech-decision` record for this repository says which one the project uses;
switch it in one pull request, together with the provider block
in `main.tf`. The bucket name is never in the file: `make init` and CI pass
it from `TF_BACKEND_BUCKET`.

## Plan

```
cp .env.example .env          # project, region, state bucket
make setup                    # tool versions, tflint plugins, git hooks
make init                     # terraform init for prod against the remote backend
make plan                     # .plans/prod.tfplan and .plans/prod.txt
make check                    # fmt, validate, lint, sec (trivy config + checkov), shell
```

`make check` prints one `<gate>: N ... checked` line per gate and a final
tally. A gate whose tool is missing prints `SKIPPED` and the tally fails;
`BEARING_ALLOW_SKIP=1 make check` lets a laptop without every scanner through
and is never set in CI.

Open a pull request. The `infra` workflow runs the same gates and
`scripts/plan.sh` for prod and posts the plan. Review the plan before the
diff: a destroyed or replaced stateful resource (the data disk, the dump
bucket) is the first line of the review (eng review D9).

## Apply (a person, never the agent)

1. Merge to `main`. Run the `infra` workflow by hand with `apply: prod`.
2. It waits in the `prod-apply` environment for your approval. Read the
   plan artifact first; if it differs from the pull request plan, stop.
3. Approve. The job applies that run's plan file and reads the state back.
4. A stale plan fails the apply. Re-run, review again.

There is no `make apply` and no `terraform apply` in any script on purpose.

## Add a module

1. `modules/<name>/` with `main.tf`, `variables.tf` (type, description,
   validation on every variable), `outputs.tf`, `versions.tf` (pinned).
2. `make docs` writes the inputs and outputs table into the module's
   `README.md` with terraform-docs; commit it with the module.
3. Call it from `envs/prod/main.tf`, open a pull request, review the plan,
   then apply from the manual workflow.
4. Export what the app needs from `envs/prod/outputs.tf`.

## Monitoring

No Kubernetes: Catalift runs on one VM with docker compose (ADR-0002).
Alerts are Cloud Monitoring resources declared in Terraform (ADR-0010), each
with a runbook under `docs/runbooks/` (planned).

See `AGENTS.md` and the `infra` skill for the conventions.
