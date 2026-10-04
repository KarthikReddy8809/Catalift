# ADR-0001: Use Google Cloud as the cloud provider

- Status: Accepted
- Date: 2026-10-01
- Task: none
- Deciders: Karthik Reddy
- Area: cloud
- Reversibility: cheap: nothing is deployed yet, and ADR-0002 keeps the app in docker compose on a VM, which moves to any provider with a VM

## Context

- From the request: "deployments i will use the gcloud with the vm".
- The repository has no infrastructure, no `.bearing/company.json` and no earlier ADR, so no existing provider constrains the choice.
- The PRD (docs/product/PRD.md, Constraints) fixes Postgres in Docker and local image storage, and AI calls go to OpenRouter, outside any cloud. The cloud only has to run containers and keep a disk.
- Users are sellers on Indian marketplaces (Amazon, Flipkart in the PRD problem), so a region close to India keeps the review grid responsive.

## What else was considered

| Option | Why not | Would suit |
| --- | --- | --- |
| Google Cloud (chosen) | you pay for the VM and disk whether or not it is used | a small team that wants one console, simple IAM and an India region (asia-south1, Mumbai) |
| AWS | no advantage for a single VM, and no existing AWS account or partner is named | a client or partner already on AWS |
| Hetzner or another VPS | cheapest per VM, but no India region and fewer managed services to grow into | a fixed tiny budget with someone comfortable running servers |

## Decision

We will deploy Catalift on Google Cloud in asia-south1 (Mumbai), because you chose it, nothing in the repository argues otherwise, and it offers a growth path (Cloud SQL, Cloud Storage, Cloud Run) if the single VM is outgrown.

## Consequences

- Region asia-south1 is assumed from the Indian marketplace focus; change it before the first deploy if users are elsewhere.
- Billing and IAM live in one Google Cloud project; keep the project id in the deploy docs.
- Revisit if a client requires its own cloud account or data residency outside India.

## Commits us to

Google Cloud (Compute Engine, Persistent Disk, region asia-south1)
