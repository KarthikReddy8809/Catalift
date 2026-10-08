# Decision log

One row per technology decision. The ADR holds the full reasoning; this
table is the index. `tech-decision` maintains it.

| Date | Key | Choice | Recommended | Why it was chosen | ADR | Status |
| --- | --- | --- | --- | --- | --- | --- |
| 2026-10-01 | cloud | Google Cloud, asia-south1 | Google Cloud | user's choice; no existing provider; India region for Indian sellers | ADR-0001 | Accepted |
| 2026-10-01 | compute | One Compute Engine VM with docker compose | VM with docker compose | PRD fixes Postgres in Docker and local image storage; demo scale | ADR-0002 | Accepted |
| 2026-10-01 | frontend | React + Vite SPA, TypeScript, Tailwind, shadcn/ui, TanStack Query and Table, axios | same | signed-in app with a review grid; no SEO need; static files on the VM | ADR-0003 | Accepted |
| 2026-10-01 | backend language | Go, with the AI gateway and job runner rebuilt in Go | Go | rest of the backend suits Go; gateway behaviour is small and specified by US-00-011, US-00-012 | ADR-0004 | Accepted |
| 2026-10-01 | messaging | Postgres job table, own Go code | same | transactional enqueue on the existing Postgres; about 900 jobs a launch | ADR-0005 | Accepted |
| 2026-10-01 | auth | Own login, seeded accounts, seller and reviewer roles, Postgres sessions | same | public VM spends a real budget; API must enforce reviewer-only approval | ADR-0006 | Accepted |
| 2026-10-01 | api style | REST with OpenAPI | same | one SPA client; generated types for TanStack Query | ADR-0007 | Accepted |
| 2026-10-04 | iac | Terraform, one environment, state in a versioned GCS bucket | Terraform | kit scaffolds it; makes the restore plan one command; about 8 resources | ADR-0008 | Accepted |
| 2026-10-04 | ci and delivery | GitLab CI: make check on MRs, images to Artifact Registry, manual deploy and terraform apply, Workload Identity Federation | GitLab CI | repo plan uses GitLab group paths; manual jobs match owner-only deploys | ADR-0009 | Accepted |
| 2026-10-04 | observability | Cloud Logging and Monitoring via the Ops Agent; five alerts emailed to the owner | same | alerts must outlive the VM; nothing extra on it; about 5 MB logs a launch | ADR-0010 | Accepted |
| 2026-10-04 | ci and delivery | GitHub with GitHub Actions, manual deploy and terraform apply via the prod environment, keyless WIF (supersedes ADR-0009) | GitHub Actions | owner hosts on GitHub; every ADR-0009 guarantee carries over | ADR-0011 | Accepted |
| 2026-10-07 | roles and access | Reviewer exports and sends to the seller; seller downloads sent exports and edits brand voice | same | the seller publishes the files; the approval gate holds in the export query | ADR-0012 | Accepted |
| 2026-10-08 | compute and delivery | Caddy on the VM serves the web build from the data disk and proxies /v1; server and migrate images from Artifact Registry; secrets in Secret Manager; 7 daily disk snapshots and 14 nightly dumps | Caddy | automatic HTTPS, one small config, no extra service | ADR-0013 | Accepted |
