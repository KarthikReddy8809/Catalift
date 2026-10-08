# Architecture decisions: Catalift

One row per decision, and every contradiction settled once so it is not
settled again, differently, in each file that runs into it.

## Decisions

| Id | Title | Area | Status | Reversibility |
| --- | --- | --- | --- | --- |
| ADR-0001 | Use Google Cloud as the cloud provider | cloud | Accepted | cheap: nothing deployed yet; compose on a VM moves to any provider |
| ADR-0002 | Run Catalift on a single Compute Engine VM with docker compose | compute | Accepted | cheap: containers move to Cloud Run with Cloud SQL and Cloud Storage as a deploy change |
| ADR-0003 | Use a React and TypeScript single-page app with Tailwind CSS, shadcn/ui, TanStack and axios | frontend | Accepted | awkward: every screen is built on this stack |
| ADR-0004 | Use Go for the backend, rebuilding the AI gateway and job runner in Go | backend language | Accepted | awkward: every service, the rules engine and the gateway are written in it |
| ADR-0005 | Run background jobs from a Postgres job table | messaging | Accepted | cheap: short-lived jobs, no long-lived data to migrate |
| ADR-0006 | Use our own login with seeded accounts and seller and reviewer roles | auth | Accepted | cheap: a handful of users; the role model survives a move to a hosted provider |
| ADR-0007 | Use REST with an OpenAPI contract between the web app and the API | api style | Accepted | awkward: the client layer and handlers would be rewritten |
| ADR-0008 | Use Terraform with state in Cloud Storage for the Google Cloud resources | iac | Accepted | cheap: about 8 resources; drop the state and manage by hand |
| ADR-0009 | Use GitLab CI for checks, image builds and manual deploys | ci and delivery | Superseded by ADR-0011 | cheap: jobs call make targets; only the pipeline file changes |
| ADR-0010 | Use Google Cloud Logging and Monitoring through the Ops Agent | observability | Accepted | cheap: standard logs and metrics; swap the agent and recreate five alerts |
| ADR-0011 | Host Catalift on GitHub and run CI with GitHub Actions | ci and delivery | Accepted | cheap: jobs call make targets; only the workflow files change |
| ADR-0012 | Reviewers send approved exports to the seller; the seller owns brand voice | roles and access | Accepted | cheap: role checks on four routes and the sidebar; additive columns |
| ADR-0013 | Deploy with Caddy, Artifact Registry images, Secret Manager and nightly backups | compute and delivery | Accepted | cheap: compose and Caddyfile on one VM; the images run anywhere |

## Conflicts that were settled

### Whether reviewer-only actions are a UI switch or enforced by the API

**Between:** US-00-009 (AC-US-00-009-5), US-00-007 and US-00-009 assumptions, Q-014, ADR-0006

**Decision.** The API enforces roles: a signed-in seller's approve, edit, regenerate or export request is refused with 403, and the web app does not offer those actions to sellers. The UI role switch is dropped.

**Why.** The app runs on a public VM that spends a real AI budget, and the approval gate is the product's promise; a UI-only switch would make it cosmetic.

**Settled by:** Karthik Reddy, 2026-10-01

**What now has to change to match:**
- docs/product/backlog.md, US-00-009: AC-US-00-009-5 reads "Given a signed-in seller, when they request an approval, then the API refuses it with 403 and the grid offers no approve action."
- docs/product/backlog.md, US-00-007 and US-00-009 assumptions: replace "No sign-in; a reviewer mode switch (Q-014)" with "Roles enforced by the API (ADR-0006)".
- docs/product/questions.md, Q-014: mark replaced by ADR-0006 with the date.

### Whether the Bearing components are installed or rebuilt

**Between:** Q-017, PRD Constraints (brg_llm_gateway, brg_jobs, brg_design_variants), ADR-0004, ADR-0005

**Decision.** The gateway and the job runner are rebuilt in Go following those components' design (ADR-0004, ADR-0005). `brg_design_variants` stays out of the design until someone states what it does for Catalift.

**Why.** The backend is Go, and the components are not on this machine and appear to be Python; their required behaviour is small and fully specified by US-00-011, US-00-012 and ADR-0005.

**Settled by:** Karthik Reddy, 2026-10-01

**What now has to change to match:**
- docs/product/questions.md, Q-017: decision becomes "rebuilt in Go per ADR-0004 and ADR-0005; brg_design_variants out until its purpose is known".
- docs/product/PRD.md, Constraints: the "Built on Bearing's existing components" line notes the Go rebuild (via `prd`).
