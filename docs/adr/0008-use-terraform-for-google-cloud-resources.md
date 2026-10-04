# ADR-0008: Use Terraform with state in Cloud Storage for the Google Cloud resources

- Status: Accepted
- Date: 2026-10-04
- Task: none
- Deciders: Karthik Reddy
- Area: iac
- Reversibility: cheap: about 8 resources; dropping Terraform later means deleting the state and managing them by hand

## Context

- ADR-0001 and ADR-0002 call for one Compute Engine VM, a persistent disk with a snapshot schedule, a Cloud Storage bucket for nightly dumps, a firewall rule, a DNS record and a service account: about 8 resources.
- The HLD's restore plan (sections 4 and 7) puts a disk snapshot into a new VM; that only works if everything around the disk can be recreated quickly.
- The repo plan's CataliftInfra uses the kit's `infra` stack: Terraform 1.9 with GCS or S3 state, tflint, trivy and checkov (docs/architecture/repo-plan.json).
- Not in the repository: the number of environments. Assumed: one (a demo environment), no separate dev and prod.

## What else was considered

| Option | Why not | Would suit |
| --- | --- | --- |
| Terraform, GCS state (chosen) | a state bucket bootstrapped by hand once; HCL to learn | anything that must be rebuilt by someone other than its author |
| OpenTofu | the kit and its checks target Terraform | a team that needs the open licence |
| `gcloud` script in the repo | no state: console changes go unnoticed; updates and teardown written by hand | a VM that lives for one demo week and is never rebuilt |
| Console by hand plus a runbook | nothing reproducible | a one-off experiment |

## Decision

We will define every Google Cloud resource for Catalift in Terraform in the CataliftInfra repository, with one environment and remote state in a versioned Cloud Storage bucket created once by hand, because the kit scaffolds it with its checks, it makes the restore plan a single command, and at about 8 resources it stays small.

## Consequences

- The state bucket is created by hand before the first apply, with object versioning on; its name goes in the infra README.
- What runs inside the VM (docker compose, proxy config, env files) is not Terraform's job; the deploy pipeline (ci decision, pending) copies it.
- The kit's kustomize part is unused (repo-plan stack_note).
- Revisit if a second environment is needed (then a directory per environment) or the VM is replaced by managed containers.

## Commits us to

Terraform 1.9, Google provider for Terraform, Cloud Storage (state bucket with versioning), tflint, trivy, checkov
