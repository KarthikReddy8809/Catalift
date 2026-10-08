# API: Catalift API v1.7.0

Generated from `api/openapi.yaml` by openapi-spec (scripts/api_doc.py). Edit the spec, not this file.

**Style:** REST over HTTPS with JSON bodies, plus multipart for CSV and image uploads and text/csv for export downloads; one Go service exposing five areas (auth, catalogue, listings, export, cost) under one /v1. · **Base path:** `/v1` · **Versioning:** Major in the path (/v1, ADR-0007); dated behaviour changes inside a major in X-API-Version, server default the newest (2026-10-04).

**Authentication.** A server-side session in an HTTP-only secure cookie (catalift_session) set by POST /v1/sessions, with an X-CSRF-Token header on every state-changing request (ADR-0006). Roles seller and reviewer are checked on every route; a seller calling a reviewer action gets 403 forbidden_role. Public operations: POST /sessions, GET /healthz.

**Errors.** Every 4xx and 5xx returns the error envelope; clients switch on `code`.

| Code | HTTP status | Meaning |
| --- | --- | --- |
| `validation_failed` | 400 | Malformed request, an unknown filter, or a field failed validation; details names the field. |
| `unauthorized` | 401 | No session, an expired one, or wrong email or password at sign-in. |
| `csrf_failed` | 403 | X-CSRF-Token missing or not this session's token. |
| `forbidden_role` | 403 | Signed in, but this action needs the reviewer role (AC-US-00-009-5). |
| `not_found` | 404 | No such record. |
| `idempotency_conflict` | 409 | Idempotency-Key reused with a different body. |
| `version_conflict` | 409 | The listing or attributes changed since the version sent; reload and retry. |
| `payload_too_large` | 413 | CSV over 5 MB or an image over 10 MB. |
| `unsupported_media_type` | 415 | Not CSV, or an image that is not JPEG, PNG or WebP. |
| `budget_blocked` | 422 | AI calls are blocked until the owner clears the budget block (D13). |
| `listing_not_generated` | 422 | The listing has no generated text yet, so a field cannot be regenerated. |
| `no_approved_listings` | 422 | No channel has an approved listing, so no file would be written (AC-US-00-010-5). |
| `unprocessable` | 422 | Valid shape but the content breaks a rule; details names it. |
| `voice_note_required` | 422 | A brand in scope has no voice note and neutral_voice_confirmed is false (AC-US-00-003-5). |
| `rate_limited` | 429 | Too many requests; wait Retry-After seconds. |
| `internal` | 500 | Unexpected failure; quote request_id to support. |

## Conventions

1. Paths are plural kebab-case nouns and a non-CRUD action is a sub-resource (`POST /invoices/{id}/send`). Why: a client can guess the URL of a resource it has not seen, and verbs in paths multiply without limit.
2. JSON fields are snake_case, ids are strings, timestamps are RFC 3339 UTC with `Z`. Why: one casing and one clock remove a whole class of client parsing bugs.
3. Money is an integer `<name>_minor` plus an ISO 4217 `currency`. Why: floats cannot hold 0.10 exactly, and an amount without a currency is ambiguous.
4. Every 4xx and 5xx returns the one error envelope with a stable `code` and a `request_id`. Why: clients switch on `code`, not on English, and support finds the log line from `request_id`.
5. A record the caller may not see is 404, never 403. Why: a 403 confirms the record exists, which turns a guessed id into an oracle.
6. Every list is cursor-paginated with a capped `limit`. Why: offsets skip or repeat rows while data changes, and an uncapped page is a denial of service.
7. Every POST that creates or charges requires `Idempotency-Key`; the same key and body replays the first response for 24 hours. Why: mobile networks retry, and a retry must not create a second record or a second charge.
8. The path carries the major version and `X-API-Version` carries dated changes inside it. Why: installed clients cannot be forced to upgrade, so breaking changes need a new major and everything else a date.
9. Removal is `deprecated: true` plus a `Sunset` header and at least 90 days. Why: a removed field breaks a client nobody told, and oasdiff can only warn about what is still in the spec.
10. Every response carries the rate-limit headers and a 429 carries `Retry-After`. Why: a client that can see its budget backs off before it is throttled.
11. The base path is /v1, not /api/v1. Why: ADR-0007 and HLD section 5 fixed /v1; the reverse proxy forwards /v1 and serves the web app at /.
12. Authentication is a session cookie plus X-CSRF-Token, not a bearer JWT. Why: ADR-0006 chose server-side sessions for a handful of seeded users, revocable at once; the CSRF header closes the cookie's cross-site risk.
13. A seller calling a reviewer-only operation gets 403 forbidden_role, not 404. Why: AC-US-00-009-5 and ADR-0006 require a visible refusal; one organisation per deployment means every record is visible to every signed-in user, so 403 leaks nothing.
14. Ids are decimal strings of the database's bigint ids. Why: The data model uses bigint identity ids (data-model deviation 1); strings keep the style's id type and avoid JavaScript number precision limits.
15. AI cost is an integer cost_micro_usd (millionths of a US dollar), not an amount_minor with currency. Why: OpenRouter bills only in USD and single calls cost fractions of a cent (data-model deviation 3).
16. CSV and images are uploaded as multipart through the API, not by signed URL; caps are 5 MB per CSV and 10 MB per image. Why: PRD Constraints and ADR-0002 keep files on the VM's local disk; there is no object store to sign URLs for.
17. Edits carry the listing or attribute version they were made against; a stale version is 409 version_conflict. Why: Tenet 5 and D5, D12: approval covers one exact version, so an edit made from an old screen must not land silently.
18. Rate limits are 300 requests a minute per session and 1,200 a minute per IP; sign-in is limited to 10 attempts per 15 minutes per IP (assumption, owner to confirm). Why: The style requires stated numbers; HLD section 9 asks for a sign-in rate limit; two users never come near the general limits.

## sessions

Sign in, see who is signed in, sign out (ADR-0006).

Serves ADR-0006, US-00-009, US-00-010.

| Method | Path | Does | Auth | Success | Errors |
| --- | --- | --- | --- | --- | --- |
| POST | `/sessions` | Sign in | none | 200 Signed in; Set-Cookie carries the session | 400 validation_failed, 401 unauthorized, 429 rate_limited, 500 internal |
| GET | `/sessions/current` | Who is signed in | sessionCookie | 200 The signed-in user and the CSRF token | 401 unauthorized, 500 internal |
| DELETE | `/sessions/current` | Sign out | sessionCookie | 204 Signed out; the cookie is cleared | 401 unauthorized, 403 forbidden_role, 403 csrf_failed, 500 internal |

Idempotency:

- `POST /sessions`: not idempotent; a retry repeats the action.
- `DELETE /sessions/current`: idempotent by definition; a retry has the same effect.

## brands

Brands from the CSV and their voice notes (US-00-003).

Serves US-00-003.

| Method | Path | Does | Auth | Success | Errors |
| --- | --- | --- | --- | --- | --- |
| GET | `/brands` | List brands with their voice notes | sessionCookie | 200 One page of brands, by name | 400 validation_failed, 401 unauthorized, 429 rate_limited, 500 internal |
| PATCH | `/brands/{brand_id}` | Set a brand's voice (seller) | sessionCookie | 200 The updated brand | 400 validation_failed, 401 unauthorized, 403 forbidden_role, 403 csrf_failed, 404 not_found, 422 unprocessable, 422 voice_note_required, 422 budget_blocked, 422 listing_not_generated, 422 no_approved_listings, 500 internal |

Idempotency:

- `PATCH /brands/{brand_id}`: documented idempotent (x-idempotent).

## uploads

CSV and image uploads with their summaries (US-00-001).

Serves US-00-001, US-00-011.

| Method | Path | Does | Auth | Success | Errors |
| --- | --- | --- | --- | --- | --- |
| POST | `/uploads` | Upload a product CSV | sessionCookie | 201 Upload summary | 400 validation_failed, 401 unauthorized, 403 forbidden_role, 403 csrf_failed, 409 idempotency_conflict, 409 version_conflict, 413 payload_too_large, 415 unsupported_media_type, 422 unprocessable, 422 voice_note_required, 422 budget_blocked, 422 listing_not_generated, 422 no_approved_listings, 500 internal |
| GET | `/uploads/latest` | Get the newest upload's summary | sessionCookie | 200 The newest upload's summary | 401 unauthorized, 404 not_found, 500 internal |
| GET | `/uploads/{upload_id}` | Get an upload's summary | sessionCookie | 200 The upload summary with its rejected rows and AI cost total | 401 unauthorized, 404 not_found, 500 internal |
| POST | `/uploads/{upload_id}/images` | Upload images for an upload's products | sessionCookie | 201 Images attached | 400 validation_failed, 401 unauthorized, 403 forbidden_role, 403 csrf_failed, 404 not_found, 409 idempotency_conflict, 409 version_conflict, 413 payload_too_large, 415 unsupported_media_type, 500 internal |
| GET | `/row-errors` | Rejected CSV rows not yet fixed | sessionCookie | 200 The open rejected rows | 400 validation_failed, 401 unauthorized, 500 internal |
| DELETE | `/row-errors` | Discard every rejected row (seller) | sessionCookie | 200 How many rows were discarded | 400 validation_failed, 401 unauthorized, 403 forbidden_role, 403 csrf_failed, 500 internal |
| PUT | `/uploads/{upload_id}/rows/{row_number}` | Correct a rejected row and send it again (seller) | sessionCookie | 200 Loaded, or rejected again with the reason | 400 validation_failed, 401 unauthorized, 403 forbidden_role, 403 csrf_failed, 404 not_found, 500 internal |
| DELETE | `/uploads/{upload_id}/rows/{row_number}` | Discard one rejected row (seller) | sessionCookie | 204 Discarded | 400 validation_failed, 401 unauthorized, 403 forbidden_role, 403 csrf_failed, 404 not_found, 500 internal |

Idempotency:

- `POST /uploads`: Idempotency-Key required. Client UUID. Same key and body replays the first result for 24 hours; same key and a different body is 409 idempotency_conflict.
- `POST /uploads/{upload_id}/images`: Idempotency-Key required. Client UUID. Same key and body replays the first result for 24 hours; same key and a different body is 409 idempotency_conflict.
- `DELETE /row-errors`: idempotent by definition; a retry has the same effect.
- `PUT /uploads/{upload_id}/rows/{row_number}`: idempotent by definition; a retry has the same effect.
- `DELETE /uploads/{upload_id}/rows/{row_number}`: idempotent by definition; a retry has the same effect.

## products

Products, their detected attributes and their AI cost (US-00-001, US-00-002, US-00-007, US-00-011).

Serves US-00-001, US-00-002, US-00-011, US-00-003, US-00-007.

| Method | Path | Does | Auth | Success | Errors |
| --- | --- | --- | --- | --- | --- |
| GET | `/products` | List products with detection status and AI cost | sessionCookie | 200 One page of products, by SKU | 400 validation_failed, 401 unauthorized, 429 rate_limited, 500 internal |
| GET | `/products/{product_id}` | Get one product | sessionCookie | 200 The product with its attributes, images and AI cost | 401 unauthorized, 404 not_found, 500 internal |
| PATCH | `/products/{product_id}` | Correct a product's details (seller) | sessionCookie | 200 Saved, or refused with the reason | 400 validation_failed, 401 unauthorized, 403 forbidden_role, 403 csrf_failed, 404 not_found, 422 unprocessable, 422 voice_note_required, 422 budget_blocked, 422 listing_not_generated, 422 no_approved_listings, 500 internal |
| POST | `/products/{product_id}/enrich` | Run the vision call again for one product (seller) | sessionCookie | 201 The run that tracks the call | 401 unauthorized, 403 forbidden_role, 403 csrf_failed, 404 not_found, 422 unprocessable, 422 voice_note_required, 422 budget_blocked, 422 listing_not_generated, 422 no_approved_listings, 500 internal |
| GET | `/products/{product_id}/image` | Get a product's thumbnail | sessionCookie | 200 The thumbnail | 401 unauthorized, 404 not_found, 500 internal |
| POST | `/products/{product_id}/images` | Add photos to one product (seller) | sessionCookie | 201 The photos attached and those refused | 400 validation_failed, 401 unauthorized, 403 forbidden_role, 403 csrf_failed, 404 not_found, 413 payload_too_large, 500 internal |
| PATCH | `/products/{product_id}/attributes` | Correct detected attributes (reviewer) | sessionCookie | 200 The corrected attributes | 400 validation_failed, 401 unauthorized, 403 forbidden_role, 403 csrf_failed, 404 not_found, 409 idempotency_conflict, 409 version_conflict, 422 unprocessable, 422 voice_note_required, 422 budget_blocked, 422 listing_not_generated, 422 no_approved_listings, 500 internal |

Idempotency:

- `PATCH /products/{product_id}`: documented idempotent (x-idempotent).
- `POST /products/{product_id}/enrich`: Idempotency-Key required. Client UUID. Same key and body replays the first result for 24 hours; same key and a different body is 409 idempotency_conflict.
- `POST /products/{product_id}/images`: Idempotency-Key required. Client UUID. Same key and body replays the first result for 24 hours; same key and a different body is 409 idempotency_conflict.
- `PATCH /products/{product_id}/attributes`: documented idempotent (x-idempotent).

## generation

Starting, following and resuming generation (US-00-002, US-00-003, US-00-012).

Serves US-00-002, US-00-003, US-00-012.

| Method | Path | Does | Auth | Success | Errors |
| --- | --- | --- | --- | --- | --- |
| POST | `/generation-runs` | Start generation | sessionCookie | 201 The run, with what was queued | 400 validation_failed, 401 unauthorized, 403 forbidden_role, 403 csrf_failed, 409 idempotency_conflict, 409 version_conflict, 422 unprocessable, 422 voice_note_required, 422 budget_blocked, 422 listing_not_generated, 422 no_approved_listings, 500 internal |
| GET | `/generation-runs/{run_id}` | Follow a run's progress | sessionCookie | 200 Progress counts for the run | 401 unauthorized, 404 not_found, 500 internal |
| POST | `/generation-runs/{run_id}/resume` | Resume failed and stopped work in a run | sessionCookie | 200 What was queued again | 401 unauthorized, 403 forbidden_role, 403 csrf_failed, 404 not_found, 422 unprocessable, 422 voice_note_required, 422 budget_blocked, 422 listing_not_generated, 422 no_approved_listings, 500 internal |

Idempotency:

- `POST /generation-runs`: Idempotency-Key required. Client UUID. Same key and body replays the first result for 24 hours; same key and a different body is 409 idempotency_conflict.
- `POST /generation-runs/{run_id}/resume`: not idempotent; a retry repeats the action.

## listings

The review grid, edits, regenerations and approvals (US-00-004, US-00-007, US-00-008, US-00-009).

Serves US-00-004, US-00-007, US-00-009, US-00-008.

| Method | Path | Does | Auth | Success | Errors |
| --- | --- | --- | --- | --- | --- |
| GET | `/listings` | The review grid | sessionCookie | 200 One page of grid rows | 400 validation_failed, 401 unauthorized, 429 rate_limited, 500 internal |
| GET | `/listings/{listing_id}` | Get one listing with its rule failures | sessionCookie | 200 The listing | 401 unauthorized, 404 not_found, 500 internal |
| PATCH | `/listings/{listing_id}` | Edit listing text (reviewer) | sessionCookie | 200 The listing after the edit and re-check | 400 validation_failed, 401 unauthorized, 403 forbidden_role, 403 csrf_failed, 404 not_found, 409 idempotency_conflict, 409 version_conflict, 422 unprocessable, 422 voice_note_required, 422 budget_blocked, 422 listing_not_generated, 422 no_approved_listings, 500 internal |
| GET | `/listings/{listing_id}/regeneration-requests` | A listing's regeneration requests | sessionCookie | 200 One page of requests, newest first | 400 validation_failed, 401 unauthorized, 404 not_found, 500 internal |
| POST | `/listings/{listing_id}/regeneration-requests` | Regenerate one field with an instruction (reviewer) | sessionCookie | 201 The queued request | 400 validation_failed, 401 unauthorized, 403 forbidden_role, 403 csrf_failed, 404 not_found, 409 idempotency_conflict, 409 version_conflict, 422 unprocessable, 422 voice_note_required, 422 budget_blocked, 422 listing_not_generated, 422 no_approved_listings, 500 internal |
| POST | `/approvals` | Approve many listings (reviewer) | sessionCookie | 201 Approved and skipped listings | 400 validation_failed, 401 unauthorized, 403 forbidden_role, 403 csrf_failed, 409 idempotency_conflict, 409 version_conflict, 500 internal |
| POST | `/rule-checks` | Check draft listing text against its channel's rules (reviewer) | sessionCookie | 200 The rules' verdict on the draft | 400 validation_failed, 401 unauthorized, 403 forbidden_role, 403 csrf_failed, 404 not_found, 500 internal |

Idempotency:

- `PATCH /listings/{listing_id}`: documented idempotent (x-idempotent).
- `POST /listings/{listing_id}/regeneration-requests`: Idempotency-Key required. Client UUID. Same key and body replays the first result for 24 hours; same key and a different body is 409 idempotency_conflict.
- `POST /approvals`: Idempotency-Key required. Client UUID. Same key and body replays the first result for 24 hours; same key and a different body is 409 idempotency_conflict.
- `POST /rule-checks`: not idempotent; a retry repeats the action.

## channels

Configured channels, their rules and re-check counts (US-00-005, US-00-006).

Serves US-00-005, US-00-006, US-00-004.

| Method | Path | Does | Auth | Success | Errors |
| --- | --- | --- | --- | --- | --- |
| GET | `/channels` | Configured channels, their rules and the last re-check | sessionCookie | 200 One page of channels | 400 validation_failed, 401 unauthorized, 500 internal |
| PATCH | `/channels/{channel}` | Change a channel's rules (reviewer) | sessionCookie | 200 The re-check the change caused | 400 validation_failed, 401 unauthorized, 403 forbidden_role, 403 csrf_failed, 404 not_found, 409 idempotency_conflict, 409 version_conflict, 422 unprocessable, 422 voice_note_required, 422 budget_blocked, 422 listing_not_generated, 422 no_approved_listings, 500 internal |

Idempotency:

- `PATCH /channels/{channel}`: documented idempotent (x-idempotent).

## exports

One CSV per channel of approved listings (US-00-010).

Serves US-00-010.

| Method | Path | Does | Auth | Success | Errors |
| --- | --- | --- | --- | --- | --- |
| GET | `/exports` | List exports, newest first | sessionCookie | 200 One page of exports | 400 validation_failed, 401 unauthorized, 500 internal |
| POST | `/exports` | Export approved listings (reviewer) | sessionCookie | 201 The export and its files | 401 unauthorized, 403 forbidden_role, 403 csrf_failed, 409 idempotency_conflict, 409 version_conflict, 422 unprocessable, 422 voice_note_required, 422 budget_blocked, 422 listing_not_generated, 422 no_approved_listings, 500 internal |
| GET | `/exports/{export_id}` | Get an export and its files | sessionCookie | 200 The export | 401 unauthorized, 403 forbidden_role, 403 csrf_failed, 404 not_found, 500 internal |
| POST | `/exports/{export_id}/send` | Send an export to the seller (reviewer) | sessionCookie | 200 The sent export | 401 unauthorized, 403 forbidden_role, 403 csrf_failed, 404 not_found, 500 internal |
| GET | `/exports/{export_id}/files/{channel}` | Download one channel's CSV | sessionCookie | 200 The CSV file | 401 unauthorized, 403 forbidden_role, 403 csrf_failed, 404 not_found, 500 internal |

Idempotency:

- `POST /exports`: Idempotency-Key required. Client UUID. Same key and body replays the first result for 24 hours; same key and a different body is 409 idempotency_conflict.
- `POST /exports/{export_id}/send`: not idempotent; a retry repeats the action.

## budget

AI spend against the USD 8 limit and the blocked state (US-00-011, US-00-012).

Serves US-00-011, US-00-012.

| Method | Path | Does | Auth | Success | Errors |
| --- | --- | --- | --- | --- | --- |
| GET | `/budget` | AI spend, the limit and whether calls are blocked | sessionCookie | 200 The budget state | 401 unauthorized, 500 internal |

## health

Liveness for the uptime check (HLD section 10).

Serves ADR-0010.

| Method | Path | Does | Auth | Success | Errors |
| --- | --- | --- | --- | --- | --- |
| GET | `/healthz` | Liveness | none | 200 The API and its database answer | 429 rate_limited, 500 internal |
