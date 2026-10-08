# ADR-0006: Use our own login with seeded accounts and seller and reviewer roles

- Status: Accepted; partly superseded by ADR-0012 (export access and brand voice)
- Date: 2026-10-01
- Task: none
- Deciders: Karthik Reddy
- Area: auth
- Reversibility: cheap: a handful of seeded users and server sessions; moving to a hosted identity provider later replaces the login screen and the session check, not the role model

## Context

- The app runs on a public Google Cloud VM (ADR-0001, ADR-0002) and every upload spends from the shared USD 8 AI budget (REQ-021, US-00-012).
- The brief's promise is that "a human signs off": approval and export are reviewer actions (US-00-009, US-00-010), and a seller must not approve (US-00-009 AC-5).
- Q-014 (docs/product/questions.md) assumed no sign-in and a role switch in the UI; that assumption predates the public deploy.
- Not in the repository: the number of users. Assumed: a handful of named people, created by the team, with no self sign-up.

## What else was considered

| Option | Why not | Would suit |
| --- | --- | --- |
| Own login with roles (chosen) | about 1 to 2 days of work: password hashing, session cookie, CSRF protection, a seed command; we own password storage | a small internal tool with two roles and a handful of users |
| No sign-in, UI role switch | anyone with the URL can upload and spend the budget; the approval gate is cosmetic | a local-only demo never deployed |
| Shared password at the reverse proxy plus UI role switch | approvals are not tied to a person and the API does not enforce roles | a demo that lives a few days on a private URL |
| Google Identity-Aware Proxy | needs a Google Cloud load balancer in front of the VM (about USD 18 a month, estimate) and app-side role mapping anyway | a team that already uses Google accounts and wants no passwords |

## Decision

We will build a simple login: user accounts with a role of seller or reviewer, created by a seed command, passwords stored hashed, and server-side sessions in Postgres carried by a secure HTTP-only cookie; the API checks the role on every route, so approve, regenerate, edit and export are refused for sellers, because the deploy is public, the budget is real, and the approval gate only means something if the API enforces it.

## Consequences

- Q-014's assumption ("no authentication for the demo") is replaced by this decision; the register entry should be updated to point here.
- Approvals record the reviewer who made them (supports US-00-009 AC-4).
- Every state-changing request needs CSRF protection, since the session is a cookie.
- No password reset or sign-up flow: the team resets passwords with the seed command.
- Revisit if users sign themselves up, if a client asks for single sign-on, or if more than about 20 users need accounts.

## Commits us to

PostgreSQL (sessions and users, existing), a password hashing library chosen in the LLD (argon2id or bcrypt), secure HTTP-only session cookies
