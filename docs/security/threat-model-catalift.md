# Threat model: Catalift

- Task: none
- Serves: US-00-001 to US-00-012, REQ-008, REQ-015, REQ-021, ADR-0002, ADR-0005, ADR-0006, ADR-0007, ADR-0009, ADR-0010
- HLD: docs/design/catalift-hld.md (v3); diagram: none yet (docs/architecture/diagrams not drawn), so boundaries come from the HLD and are marked assumption:
- Spec: api/openapi.yaml; data model: docs/design/data-model.md and docs/design/schema.sql
- Sensitive classes: auth, PII, external input
- Author: Karthik Reddy, 2026-10-04, status Draft (design stage: no code exists, so every mitigation is planned; nothing is mitigated yet)

Scope: the whole system as designed. One Go API and worker, one Postgres, a React web app behind a reverse proxy on one Google Cloud VM, OpenRouter as the AI provider, GitLab CI deploying through Workload Identity Federation. Sensitive classes found: auth (sessions, passwords, roles), PII (user emails, password hashes), external input (CSV and image uploads, voice notes and instructions that reach prompts, AI model output, channel config files). No payments; AI spend is guarded as an asset in its own right. Abuse-path pass: not run (design only, no code to anchor abuse paths; the security-threat-model skill is not installed).

## 1. Assets

| Asset | Where it lives | Owner | Why an attacker wants it |
| --- | --- | --- | --- |
| OpenRouter API key | root-only env file on the VM, read by the worker (HLD section 6) | operator rotation | spend up to the key's credit limit on someone else's account |
| AI budget (USD 8 block, USD 10 key limit) | `budget` and `ai_calls` tables; OpenRouter key limit | product owner role | exhaust it so the demo cannot run (financial denial of service) |
| Passwords and sessions | `users.password_hash`, `sessions` (hashed tokens), the catalift_session cookie | backend lead rotation | take over a reviewer account and approve or export |
| User emails | `users.email` (personal data) | backend lead rotation | phishing, credential stuffing elsewhere |
| Approval integrity | `listings.version`, `approvals`, `rule_results` | backend lead rotation | ship a listing nobody approved or that breaks marketplace rules (REQ-008, REQ-015) |
| Unreleased catalogue | `products`, `product_images` on the data disk, prompts in `ai_calls` | product owner role | a seller's unreleased season, prices and photos |
| Export CSVs | export files on the data disk, downloaded by reviewers | backend lead rotation | plant formulas that run when the file is opened, or read the launch early |
| Database, data disk and dumps | Postgres on the protected data disk; nightly dumps in a Cloud Storage bucket | operator rotation | read or destroy everything at once |
| Google Cloud project via the CI identity | Workload Identity Federation pool and CI service account (D10) | operator rotation | take over or delete the infrastructure |
| Channel rules | config/channels/*.yaml in the repository (REQ-017) | product owner role | loosen rules so non-compliant listings pass |

## 2. Trust boundaries

| # | From | To | Protocol | Auth on the edge | Source |
| --- | --- | --- | --- | --- | --- |
| B1 | internet (browsers) | reverse proxy on the VM | HTTPS | none at the proxy; session cookie checked by the API | assumption: HLD section 3, reverse proxy ADR still needed |
| B2 | reverse proxy | Catalift API | HTTP on the compose network | session cookie, CSRF header on writes, role per route | assumption: HLD section 3, ADR-0006 |
| B3 | API and worker | Postgres | SQL on the compose network | database password, one application role | assumption: HLD section 3, ADR-0002 |
| B4 | worker (AI gateway) | OpenRouter | HTTPS | OpenRouter API key | assumption: HLD section 6 |
| B5 | seller role | reviewer role | same API, different role | role checked per route (ADR-0006, tenet 6) | assumption: ADR-0006 |
| B6 | GitLab CI | Google Cloud project | HTTPS | Workload Identity Federation, protected main and environment prod (D10) | assumption: HLD section 6, ADR-0009 |
| B7 | VM | Cloud Storage dumps, Cloud Logging and Monitoring | HTTPS | VM service account | assumption: HLD sections 6 and 10, ADR-0010 |
| B8 | AI model output | listings, grid and export | data written to Postgres and rendered | none: output is untrusted text | assumption: HLD section 9 |

## 3. Entry points

| # | Entry point | Kind | Auth required | Boundary |
| --- | --- | --- | --- | --- |
| E1 | POST /v1/sessions (sign in) | route | none (public by necessity) | B1, B2 |
| E2 | GET and DELETE /v1/sessions/current | route | session; CSRF on DELETE | B1, B2 |
| E3 | POST /v1/uploads (CSV, multipart) | upload | session, seller or reviewer, CSRF | B1, B2 |
| E4 | POST /v1/uploads/{upload_id}/images (images, multipart) | upload | session, seller or reviewer, CSRF | B1, B2 |
| E5 | PATCH /v1/brands/{brand_id} (voice note that reaches prompts) | route | session, CSRF | B1, B2 |
| E6 | POST /v1/generation-runs and /resume (start AI spend) | route | session, CSRF | B1, B2, B4 |
| E7 | PATCH /v1/listings/{id} and /v1/products/{id}/attributes | route | session, reviewer, CSRF | B5 |
| E8 | POST /v1/listings/{id}/regeneration-requests (instruction that reaches prompts) | route | session, reviewer, CSRF | B5, B4 |
| E9 | POST /v1/approvals | route | session, reviewer, CSRF | B5 |
| E10 | POST /v1/exports and GET /v1/exports/{id}/files/{channel} | route, download | session, reviewer | B5 |
| E11 | read routes: GET brands, uploads, products, listings, channels, budget | route | session | B1, B2 |
| E12 | GET /v1/healthz | route | none (uptime check) | B1 |
| E13 | worker job consumer calling OpenRouter and storing its output | consumer | database role, API key | B3, B4, B8 |
| E14 | channel config files loaded at start-up | config | repository merge request, then deploy | B6 |
| E15 | GitLab CI manual deploy and terraform apply jobs | admin pipeline | protected branch, WIF | B6 |
| E16 | seed command and VM shell (create users, reset passwords, read env file) | admin screen | VM access | B3, B7 |

## 4. Threats (STRIDE)

| Id | Entry or boundary | Category | Threat | Likelihood | Impact | Mitigation | Status |
| --- | --- | --- | --- | --- | --- | --- | --- |
| T-01 | E1 | Spoofing | password guessing or credential stuffing against a reviewer account | M | H | new story: Sign-in with seeded accounts, argon2id hashes, 10 attempts per 15 minutes per IP (ADR-0006) | planned |
| T-02 | E1 | Information disclosure | sign-in reveals which emails exist through different errors or timing | M | L | new story: Sign-in with seeded accounts, argon2id hashes, 10 attempts per 15 minutes per IP (ADR-0006) | planned |
| T-03 | E1 | Tampering | session fixation: an attacker-set cookie survives sign-in | L | H | new story: Sign-in with seeded accounts, argon2id hashes, 10 attempts per 15 minutes per IP (ADR-0006) | planned |
| T-04 | E1 | Repudiation | failed and successful sign-ins are not recorded, so account misuse cannot be traced | M | M | new story: Log sign-in success and failure with user and IP to Cloud Logging | planned |
| T-05 | E1 | Denial of service | considered, none: password hashing cost is bounded by the sign-in rate limit in T-01 | | | | |
| T-06 | E1 | Elevation of privilege | considered, none: the role comes only from the users row created by the seed command; sign-in takes no role input | | | | |
| T-07 | E2 | Spoofing | a stolen session cookie (script injection, shared machine) is replayed | M | H | new story: HttpOnly, Secure, SameSite=Lax session cookie and a Content-Security-Policy with no inline script | planned |
| T-08 | E2 | Tampering | cross-site request forgery makes a signed-in reviewer approve or export | M | H | new story: Sign-in with seeded accounts, argon2id hashes, 10 attempts per 15 minutes per IP (ADR-0006) | planned |
| T-09 | E2 | Information disclosure | a database read hands out live sessions | L | H | sessions store only a hash of the token, docs/design/schema.sql:50 | planned |
| T-10 | E3 | Tampering | a CSV row overwrites another seller's product by reusing its SKU | M | M | US-00-001 | planned |
| T-11 | E3 | Denial of service | an oversized or row-heavy CSV exhausts memory or the request worker | M | M | new story: Body size caps at the proxy and API (5 MB CSV, 10 MB image, 1 MB JSON) with 413 payload_too_large | planned |
| T-12 | E3 | Information disclosure | a parser error returns stack traces, SQL or file paths | M | L | new story: One error handler returning the error envelope only, never stack traces, SQL or paths | planned |
| T-13 | E3 | Repudiation | considered, none: every upload records who made it, docs/design/schema.sql:94 | | | | |
| T-14 | E3 | Elevation of privilege | considered, none: both roles may upload (US-00-001); the route needs a session | | | | |
| T-15 | E4 | Tampering | a crafted file (polyglot, script, path traversal in its name) is stored and later served | M | H | new story: Sniff image type, accept only JPEG, PNG and WebP, store under generated names | planned |
| T-16 | E4 | Denial of service | a decompression bomb exhausts memory when the 1024 px detection copy is made (D23) | M | M | new story: Reject images whose decoded size exceeds 50 megapixels before resizing | planned |
| T-17 | E4 | Information disclosure | photo metadata (location, device) is sent to OpenRouter with the image | L | L | new story: Strip metadata from the 1024 px detection copy | planned |
| T-18 | E5, E8, E3 | Tampering | prompt injection in a voice note, instruction or CSV field makes the model write misleading or off-brand text | M | M | US-00-004 | planned |
| T-19 | E5, E8 | Information disclosure | considered, none: each prompt holds one product's fields and its brand note only (HLD section 9), so there is no other data to extract | | | | |
| T-20 | E6 | Denial of service | a seller or a stolen session starts runs and resumes repeatedly to burn the USD 8 budget | M | M | US-00-012 | planned |
| T-21 | E6 | Elevation of privilege | considered, none: both roles may start and resume generation (US-00-003) | | | | |
| T-22 | E7, E8, E9, E10 | Elevation of privilege | a seller calls a reviewer-only route directly and edits, regenerates, approves or exports | M | H | new story: Role check on every route with a 403 forbidden_role test for each reviewer-only operation | planned |
| T-23 | E9 | Tampering | an approval lands on a listing that changed or now fails its rules | M | H | US-00-009 | planned |
| T-24 | E7 | Repudiation | who changed a listing's text or attributes is not recorded (the data model dropped the edit history) | M | M | new story: Record who changed a listing's text and attributes, and when | planned |
| T-25 | E10 | Tampering | spreadsheet formula injection: AI text or CSV fields starting with =, +, - or @ run when the export is opened | M | H | new story: Escape spreadsheet formula prefixes in every export cell | planned |
| T-26 | E10 | Tampering | an export includes an unapproved listing or one edited after approval | M | H | US-00-010 | planned |
| T-27 | E11 | Information disclosure | every signed-in user sees every brand, product and listing; a second seller organisation on the same deployment would see the first one's launch | L | H | new story: Organisation scoping before a second seller organisation shares a deployment | unmitigated |
| T-28 | E11, B1 | Denial of service | request floods from one session or IP | M | M | new story: Rate limits of 300 a minute per session and 1,200 a minute per IP with rate-limit headers | planned |
| T-29 | E12 | Information disclosure | the public health route reveals versions or database details | L | L | the health response is only a status field, api/openapi.yaml:1351 | planned |
| T-30 | E13, B4 | Information disclosure | the OpenRouter key leaks through the image, logs or a CI variable | L | H | new story: OpenRouter key held only in a root-only file on the VM, never in images, logs or CI, with a USD 10 key limit | planned |
| T-31 | E13, B4 | Information disclosure | prompts with unreleased products are retained or used for training by the upstream provider | M | M | new story: Pin OpenRouter provider routing to providers that do not retain or train on prompts | planned |
| T-32 | E13, B8 | Tampering | model output with HTML or script is rendered in the review grid (stored cross-site scripting) | M | H | new story: Render listing text as plain text only, with a lint rule against raw HTML rendering | planned |
| T-33 | E13, B4 | Denial of service | provider outages or rate limits fail a whole run | M | M | new story: Retry policy of 3 attempts and 429 waits of up to 1 hour (eng review D14, D15) | planned |
| T-34 | E13 | Repudiation | AI spend cannot be traced to a product or purpose | L | M | US-00-011 | planned |
| T-35 | E13, B4 | Spoofing | considered, none: the gateway calls a fixed HTTPS host with certificate verification on and no plaintext fallback | | | | |
| T-36 | E14 | Tampering | a changed or broken channel rule lets non-compliant listings pass | L | H | US-00-005 | planned |
| T-37 | E15, B6 | Elevation of privilege | a pipeline from a fork or an unprotected branch obtains the CI identity and changes the project | L | H | new story: Workload Identity Federation trusting only protected main and environment prod, with named CI roles (eng review D10) | planned |
| T-38 | E15 | Denial of service | a routine terraform apply replaces the VM and destroys the data disk or dump bucket | L | H | new story: Protected data disk and dump bucket with prevent_destroy and an apply job that refuses destroying plans (eng review D9) | planned |
| T-39 | E15 | Tampering | a compromised dependency enters the build | L | H | new story: Lockfiles for Go and the web app and a dependency audit job in CI | planned |
| T-40 | E16, B7 | Elevation of privilege | anyone with shell on the VM reads the env file, the database or resets passwords | L | H | new story: VM access only through OS Login over IAP for named operators, no public SSH | planned |
| T-41 | E16 | Repudiation | seed command actions (users created, passwords reset, budget block cleared) leave no trace | L | M | new story: Log every seed command action with the operator identity to Cloud Logging | planned |
| T-42 | B1 | Information disclosure | traffic or cookies sent over plain HTTP or downgraded | L | H | new story: HTTPS only at the reverse proxy with HSTS and an HTTP to HTTPS redirect | planned |
| T-43 | B3 | Tampering | SQL injection through any input reaching a query | L | H | new story: Every query through sqlc with bound parameters; a CI check refusing string-built SQL | planned |
| T-44 | B3 | Elevation of privilege | the application connects as the database owner, so an injection can alter the schema | L | H | new story: Separate migration role and least-privilege application role in Postgres | planned |
| T-45 | B7 | Information disclosure | nightly dumps in Cloud Storage are readable by more than the operator | L | H | new story: Private dump bucket with uniform access, no public access, access only for the VM and operators | planned |
| T-46 | B7 | Information disclosure | session cookies, CSRF tokens, passwords or the API key appear in logs | M | H | new story: Log redaction for cookies, CSRF tokens, passwords and the OpenRouter key | planned |
| T-47 | B2 | Spoofing | considered, none: the API is reachable only on the compose network behind the proxy, and every route checks the session itself (ADR-0006) | | | | |

Categories: Spoofing, Tampering, Repudiation, Information disclosure, Denial of service, Elevation of privilege. Entry points and boundaries not given a row for a category were considered and share the reasons above: the read routes and downloads inherit T-07, T-08, T-22 and T-28; the boundaries B5 and B8 are covered through T-22, T-25 and T-32.

## 5. Residual risks

| Threat | Risk accepted or planned | Owner (role) | Revisit |
| --- | --- | --- | --- |
| T-01, T-02, T-03, T-08 | planned: new sign-in and session story (ADR-0006) | backend lead rotation | 2026-10-31 |
| T-04 | planned: sign-in logging story | backend lead rotation | 2026-10-31 |
| T-07 | planned: cookie flags and Content-Security-Policy story | frontend lead rotation | 2026-10-31 |
| T-09 | planned: hashed session tokens in schema.sql, built with the auth story | backend lead rotation | 2026-10-31 |
| T-10 | planned in US-00-001 (D21 rejects an existing SKU) | backend lead rotation | 2026-10-31 |
| T-11, T-12 | planned: size caps and error handler stories | backend lead rotation | 2026-10-31 |
| T-15, T-16, T-17 | planned: image handling stories | backend lead rotation | 2026-10-31 |
| T-18 | planned in US-00-004 plus human review in US-00-009; accepted that odd text can be generated, never exported unreviewed | product owner role | 2026-11-15 |
| T-20 | planned in US-00-012 plus the USD 10 key limit | product owner role | 2026-10-31 |
| T-22 | planned: route role check story | backend lead rotation | 2026-10-31 |
| T-23 | planned in US-00-009 | backend lead rotation | 2026-10-31 |
| T-24 | planned: edit attribution story (conflicts with the data model dropping edit history; decide with the HLD revision) | product owner role | 2026-10-15 |
| T-25 | planned: export escaping story | backend lead rotation | 2026-10-31 |
| T-26 | planned in US-00-010 | backend lead rotation | 2026-10-31 |
| T-27 | accepted: one seller organisation per deployment (HLD section 9); unmitigated until a second organisation is planned | product owner role | 2027-01-01 |
| T-28 | planned: rate limit story | backend lead rotation | 2026-10-31 |
| T-29 | planned: health schema in api/openapi.yaml | backend lead rotation | 2026-10-31 |
| T-30 | planned: key handling story, pending the secrets ADR | operator rotation | 2026-10-15 |
| T-31 | planned: provider routing story; accepted for drawn demo images until real seller data arrives | product owner role | 2026-11-15 |
| T-32 | planned: plain-text rendering story | frontend lead rotation | 2026-10-31 |
| T-33 | planned: retry policy (D14, D15) | backend lead rotation | 2026-10-31 |
| T-34 | planned in US-00-011 | backend lead rotation | 2026-10-31 |
| T-36 | planned in US-00-005 plus D7 re-check and D8 CI validation | product owner role | 2026-10-31 |
| T-37 | planned: WIF scope story (D10) | operator rotation | 2026-10-31 |
| T-38 | planned: data disk protection story (D9) | operator rotation | 2026-10-31 |
| T-39 | planned: lockfiles and dependency audit story | operator rotation | 2026-11-15 |
| T-40, T-41 | planned: VM access and seed logging stories | operator rotation | 2026-10-31 |
| T-42 | planned: HTTPS and HSTS story, pending the reverse proxy ADR | operator rotation | 2026-10-15 |
| T-43, T-44 | planned: sqlc and database roles stories | backend lead rotation | 2026-10-31 |
| T-45 | planned: private dump bucket story, pending the backups ADR | operator rotation | 2026-10-15 |
| T-46 | planned: log redaction story | backend lead rotation | 2026-10-31 |

## 6. New stories needed

- Sign-in with seeded accounts, argon2id hashes, 10 attempts per 15 minutes per IP (ADR-0006) (mitigates T-01, T-02, T-03, T-08), acceptance: an 11th sign-in attempt from one IP within 15 minutes gets 429; wrong email and wrong password return the same 401 in similar time; the session id changes at sign-in; a write without the session's CSRF token gets 403 csrf_failed.
- Log sign-in success and failure with user and IP to Cloud Logging (mitigates T-04), acceptance: one log line per attempt with outcome, user id when known, and IP; no password or token in it.
- HttpOnly, Secure, SameSite=Lax session cookie and a Content-Security-Policy with no inline script (mitigates T-07), acceptance: the Set-Cookie header carries all three flags and every HTML response carries a CSP without unsafe-inline.
- Body size caps at the proxy and API (5 MB CSV, 10 MB image, 1 MB JSON) with 413 payload_too_large (mitigates T-11), acceptance: a 6 MB CSV gets 413 before the API reads it all.
- One error handler returning the error envelope only, never stack traces, SQL or paths (mitigates T-12), acceptance: a forced panic returns 500 internal with a request_id and nothing else.
- Sniff image type, accept only JPEG, PNG and WebP, store under generated names (mitigates T-15), acceptance: an HTML file renamed .jpg gets 415, and a stored file's name contains no part of the uploaded name.
- Reject images whose decoded size exceeds 50 megapixels before resizing (mitigates T-16), acceptance: a 20,000 by 20,000 pixel PNG gets 422 without being decoded in full.
- Strip metadata from the 1024 px detection copy (mitigates T-17), acceptance: the detection copy of a photo with GPS metadata has none.
- Role check on every route with a 403 forbidden_role test for each reviewer-only operation (mitigates T-22), acceptance: a seller session gets 403 on every operation marked reviewer in api/openapi.yaml, tested per operation.
- Record who changed a listing's text and attributes, and when (mitigates T-24), acceptance: after an edit, the listing shows the reviewer and time of its last change.
- Escape spreadsheet formula prefixes in every export cell (mitigates T-25), acceptance: a title starting with = is exported starting with a single quote.
- Organisation scoping before a second seller organisation shares a deployment (mitigates T-27), acceptance: written as a REQ through prd before any second organisation is onboarded.
- Rate limits of 300 a minute per session and 1,200 a minute per IP with rate-limit headers (mitigates T-28), acceptance: the 301st request in a minute from one session gets 429 with Retry-After.
- OpenRouter key held only in a root-only file on the VM, never in images, logs or CI, with a USD 10 key limit (mitigates T-30), acceptance: a search of images, logs and CI variables finds no key; the key's limit reads USD 10 in OpenRouter.
- Pin OpenRouter provider routing to providers that do not retain or train on prompts (mitigates T-31), acceptance: every gateway request carries the provider preference, verified in the request log.
- Render listing text as plain text only, with a lint rule against raw HTML rendering (mitigates T-32), acceptance: a title containing a script tag shows as text in the grid, and the lint fails on any raw HTML rendering.
- Retry policy of 3 attempts and 429 waits of up to 1 hour (eng review D14, D15) (mitigates T-33), acceptance: tests show a job failing after its third 5xx and surviving a 20 minute 429 burst.
- Workload Identity Federation trusting only protected main and environment prod, with named CI roles (eng review D10) (mitigates T-37), acceptance: a pipeline on an unprotected branch cannot obtain a token.
- Protected data disk and dump bucket with prevent_destroy and an apply job that refuses destroying plans (eng review D9) (mitigates T-38), acceptance: a plan that replaces the data disk fails the apply job.
- Lockfiles for Go and the web app and a dependency audit job in CI (mitigates T-39), acceptance: CI fails on a known critical vulnerability in a locked dependency.
- VM access only through OS Login over IAP for named operators, no public SSH (mitigates T-40), acceptance: port 22 is closed to the internet and an unlisted account cannot open a shell.
- Log every seed command action with the operator identity to Cloud Logging (mitigates T-41), acceptance: creating a user and clearing the budget block each produce one log line naming the operator.
- HTTPS only at the reverse proxy with HSTS and an HTTP to HTTPS redirect (mitigates T-42), acceptance: an HTTP request gets a redirect and every HTTPS response carries Strict-Transport-Security.
- Every query through sqlc with bound parameters; a CI check refusing string-built SQL (mitigates T-43), acceptance: the CI check fails on a query built with string formatting.
- Separate migration role and least-privilege application role in Postgres (mitigates T-44), acceptance: the application role cannot run DDL.
- Private dump bucket with uniform access, no public access, access only for the VM and operators (mitigates T-45), acceptance: an anonymous read of a dump object is refused.
- Log redaction for cookies, CSRF tokens, passwords and the OpenRouter key (mitigates T-46), acceptance: a test request with each secret produces log lines that contain none of them.

## 7. Counts

Assets 10, boundaries 8 (all assumption:, no C4 diagram yet), entry points 16. From threats_check.py:

- threat-model: 1 files, 39 threats (mitigated 0, planned 38, unmitigated 1; retired 0), mitigations: code 2, story 7, new story 30; 3 sensitive classes, 0 problems
- Threats by category: S 2, T 12, R 4, I 11, D 6, E 4
- Gate: passed
