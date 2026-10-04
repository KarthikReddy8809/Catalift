# ADR-0002: Run Catalift on a single Compute Engine VM with docker compose

- Status: Accepted
- Date: 2026-10-01
- Task: none
- Deciders: Karthik Reddy
- Area: compute
- Reversibility: cheap: the services are containers; moving to Cloud Run plus Cloud SQL plus Cloud Storage is a deploy change, plus moving images off local disk

## Context

- From the request: "gcloud with the vm".
- The PRD fixes Postgres in Docker and product images on local storage (docs/product/PRD.md, Constraints). Both need a persistent local disk, which a VM has and request-scoped containers (Cloud Run) do not.
- Load is small: a 300-SKU launch and two personas (PRD section 4); generation runs as background jobs (US-00-003) that must keep running after the HTTP response, which a VM process does.
- AI spend is capped at USD 10 (PRD Constraints), so the product is a demo-scale build, not a high-availability service.

## What else was considered

| Option | Why not | Would suit |
| --- | --- | --- |
| One VM with docker compose (chosen) | one machine is a single point of failure; no rolling deploys; backups and OS patching are ours | one or two services, a fixed small budget, and Postgres and images on local disk as the PRD states |
| Cloud Run plus Cloud SQL plus Cloud Storage | breaks the PRD's "Postgres in Docker, local image storage"; the background worker needs always-on instances; more pieces to set up for a demo | when uptime matters, traffic is spiky, or more than one instance is needed |
| GKE Autopilot | far more setup and cost than one app needs | many services, several teams |

## Decision

We will run the API, the background worker, Postgres and a reverse proxy serving the built frontend as docker compose services on one Compute Engine VM with a persistent disk, because it matches the PRD's storage constraints, keeps background jobs simple, and is the cheapest setup at this scale.

## Consequences

- Postgres data and product images live on the VM's persistent disk: schedule daily disk snapshots and a `pg_dump` to Cloud Storage, or a lost disk loses everything.
- Deploys restart containers, with a short outage; acceptable for a demo.
- HTTPS comes from the reverse proxy (Caddy with automatic certificates, or Nginx with certbot), decided when the deploy is built.
- The OpenRouter key lives on the VM; keep it in Secret Manager or a root-only env file, never in the image.
- Revisit when any of these holds: more than one instance is needed, an uptime commitment is made to a client, or images pass the disk size chosen at setup.

## Commits us to

Compute Engine VM, Persistent Disk with snapshot schedule, Docker, docker compose, PostgreSQL in a container, a reverse proxy (Caddy or Nginx, chosen at deploy time)
