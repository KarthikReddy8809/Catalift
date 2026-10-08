# ADR-0013: Deploy with Caddy, Artifact Registry images, Secret Manager and nightly backups

- Status: Accepted
- Date: 2026-10-08
- Task: US-00-011
- Deciders: Karthik Reddy
- Area: compute and delivery
- Reversibility: cheap: everything is a compose file, a Caddyfile and a script on one VM; the images and the web build run anywhere
- Supersedes: none; settles the three "ADR needed" items of ADR-0002 and HLD section 15 (reverse proxy, secrets, backups)

## Context

- From the request (2026-10-08): "use the caddy as the reverse proxy and caddy serve the frontend build stored in the vm and the backend is fetched as the image in the artifact registry and the migrations also the as the image".
- ADR-0002 runs everything on one Compute Engine VM with docker compose and left the reverse proxy, the secrets store and the backup policy open.
- HLD section 12: migrations run once per deploy before the API and the worker; the worker needs 75 seconds to stop (eng review D16).
- Scale: one environment, about 10 users a minute.
- Retention chosen by the owner on 2026-10-08: 7 daily disk snapshots, 14 nightly database dumps. No domain yet.

## What else was considered

| Option | Why not | Would suit |
| --- | --- | --- |
| Caddy serving the web build from the data disk (chosen) | the web build is not an image, so it needs its own upload and switch step | one VM, automatic HTTPS, the smallest config |
| Nginx with certbot | a second process and renewal timer for certificates | a team that already runs Nginx |
| The web build as an Nginx image behind Caddy | two web servers in a row for static files | several frontends on one host |
| Secrets in a root-only file copied by hand | no audit trail, no rotation without a login | a host with no cloud secret store |
| A domain and DNS zone now | no domain exists yet | later: set `domain`, point the A record at `vm_ip` |

## Decision

Caddy is the only published container on the VM: it serves HTTPS with automatic certificates (for `<ip>.sslip.io` until a domain is set), serves the web build from `/mnt/data/web/current` with an `index.html` fallback, and forwards `/v1`, `/healthz` and `/readyz` to the API. CI builds two images per commit, `server` (the API and the worker binaries) and `migrate` (goose and the migration files), pushes them to Artifact Registry, and uploads the web build to a bucket. `deploy.sh TAG` on the VM reads the OpenRouter key and the Postgres password from Secret Manager into a root-only env file, runs the migrations, switches the web build and restarts the API, the worker and Caddy; rollback is the same script with an earlier tag. The data disk has a daily snapshot kept 7 days; a nightly `pg_dump` goes to a bucket that deletes it after 14 days.

## Consequences

- The data disk and the dump bucket carry `prevent_destroy`; the VM has deletion protection.
- Product images are only in the disk snapshots, not in the dumps (HLD section 4): a restore older than 7 days loses images.
- SSH is only through IAP with OS Login; no port 22 is open to the internet.
- The CI service account and Workload Identity Federation come from the bootstrap step (ADR-0011); Terraform grants that account only what deploys need, on the VM, the registry and the web bucket.
- Revisit when an uptime commitment is made, traffic needs a second VM, or images outgrow the data disk.

## Commits us to

Caddy 2, Artifact Registry (Docker), Secret Manager, Cloud Storage for web builds and dumps, Compute Engine snapshot schedules, IAP TCP forwarding with OS Login, Debian 12 with Docker Engine and the compose plugin.
