# ADR-0009: Use GitLab CI for checks, image builds and manual deploys

- Status: Superseded by ADR-0011
- Date: 2026-10-04
- Task: none
- Deciders: Karthik Reddy
- Area: ci and delivery
- Reversibility: cheap: every job calls a make target, so moving to another CI means rewriting the pipeline file, not the build

## Context

- Three repositories: CataliftClient, CataliftApi, CataliftInfra (docs/architecture/repo-plan.json), with GitLab-style group and subgroup paths (`Catalift/Client/...`).
- Each kit stack exposes `make check`; tests replay recorded AI responses, so CI spends nothing on AI (PRD Constraints, US-00-012 AC-4).
- Runtime is docker compose on one VM (ADR-0002); infrastructure is Terraform (ADR-0008).
- The owner's standing rule: the owner pushes, merges and deploys; nothing deploys automatically.
- Not in the repository: the git host (no remote yet). Decided here: GitLab.

## What else was considered

| Option | Why not | Would suit |
| --- | --- | --- |
| GitLab CI (chosen) | needs a runner, shared or own | code on GitLab, group and subgroup layout, manual protected deploys |
| GitHub Actions | the subgroup paths in the repo plan do not map to GitHub | code on GitHub |
| No CI, `make deploy` from a laptop | checks run only when remembered; deploys depend on one machine and its credentials | a one-person throwaway prototype |

## Decision

We will host the three repositories on GitLab under the `Catalift` group and run GitLab CI in each: `make check` on every merge request (lint, tests on replayed AI responses, the OpenAPI drift check in CataliftApi); on `main`, build and push images to Artifact Registry; a manual deploy job that connects to the VM and pulls and restarts the compose stack; and in CataliftInfra a `terraform plan` on merge requests with a manual `apply` job. CI authenticates to Google Cloud through Workload Identity Federation, with no stored service account key. We chose it because it matches the repo plan and the kit's pipeline templates, and manual jobs match the owner's rule that only the owner deploys.

## Consequences

- Artifact Registry (one Docker repository in asia-south1) is added to the Terraform resources (ADR-0008).
- The deploy job needs SSH or IAP access to the VM from the runner; the method is set in the LLD.
- No job runs with a live OpenRouter key; the live key stays on the VM only.
- Revisit if the code moves to GitHub, or if more than one environment needs promotion between them.

## Commits us to

GitLab, GitLab CI, Artifact Registry, Workload Identity Federation
