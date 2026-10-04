# Data model: Catalift

**Store:** PostgreSQL · **Tables:** 19 · **Columns:** 161 · **Indexes:** 31 · **Personal-data columns:** 2

One PostgreSQL 16 database in Docker on the VM's protected data disk holds everything Catalift knows: seeded users and their sessions, the catalogue (brands, uploads, products, images), the listings area (attributes, listings, rule results, approvals, regeneration requests), the platform job table, the AI cost ledger with its budget row, and exports. The Go API writes it through five areas that each own their tables (HLD v3, tenet 1); the Go worker claims jobs from it and writes results through the owning area. One relational store is enough: every rule is a transaction over a few thousand rows, and the job queue lives in the same database so enqueueing commits with the change it belongs to (ADR-0005).

- Task: none
- Serves: US-00-001 to US-00-012; REQ-001 to REQ-021; ADR-0002, ADR-0005, ADR-0006; eng review D5, D6, D7, D12, D13, D14, D15, D19, D20, D21, D23
- ADRs: ADR-0002 and ADR-0005 (PostgreSQL in Docker; job table in Postgres)
- Stores: postgres
- Companion files: `docs/design/schema.sql`, `docs/design/data-dictionary.csv`, `docs/design/erd.md`
- Author: Karthik Reddy (git config user.name), 2026-10-04, status Draft, version v1

## 1. Why these stores

### PostgreSQL

**Holds:** every table in this document, including the job queue and the cost ledger.

Every acceptance criterion is relational and consistency-critical: an approval must match the exact listing version it covers (REQ-015, tenet 5), an attribute fix must bump versions and clear approvals in one transaction (D5), and a job must be enqueued in the same transaction as the state change it serves (ADR-0005). The volumes are small: about 600 listings and 900 AI calls per launch, and the USD 8 budget allows about two launches (HLD section 8), so every table stays under 10^4 rows.

| Considered | Why not |
| --- | --- |
| MongoDB | The shapes are fixed and cross-referenced (product, listing, approval, job); the stories need foreign keys, unique keys and multi-row transactions, not whole documents. |
| ClickHouse | Listings, approvals and jobs are edited records with status changes, not append-only events; the success measures (HLD section 11) aggregate a few thousand rows on demand. |
| Redis or a broker for jobs | ADR-0005 chose the Postgres job table so enqueue commits with the state change; a second store would lose that. |

## 2. Entities: ownership and lifecycle

Areas follow HLD v3: auth, catalogue, listings, export, cost, plus the platform job table. Every writer path is named; a NOT NULL column's value comes from the path that creates the row.

| Entity | Owner | Created by | Changed by | Ended by | Stories | PII | Retention |
| --- | --- | --- | --- | --- | --- | --- | --- |
| user | auth | seed command (ADR-0006) | seed command (password reset, erasure) | UNDEFINED; erasure keeps the row (section 11) | ADR-0006, US-00-009, US-00-010 | yes | UNDEFINED |
| session | auth | login | none | daily sweep after expires_at | ADR-0006 | no | until expiry; session length UNDEFINED |
| brand | catalogue | CSV upload (first time a brand name appears) | seller sets the voice note | not ended | US-00-001, US-00-003 | no | UNDEFINED |
| upload | catalogue | CSV upload | none after the summary is written | not ended | US-00-001 | no | UNDEFINED |
| upload row error | catalogue | CSV upload | none | with its upload | US-00-001 | no | as its upload |
| product | catalogue | CSV upload (valid row, new SKU, D21) | none (no edit story; TODOS.md D28) | not ended | US-00-001, US-00-002, US-00-003, US-00-011 | no | UNDEFINED |
| product image | catalogue | image upload (D20 match) | none | not ended | US-00-001, US-00-002 | no | UNDEFINED |
| product attributes | listings | generation start (pending) | worker detection (done, failed or stopped_budget; revision raised), reviewer correction (D5; revision raised) | not ended | US-00-002, US-00-007 | no | UNDEFINED |
| generation run | listings | seller starts generation | none | not ended | US-00-003, US-00-012 | no | UNDEFINED |
| listing | listings | generation start or resume (queued, D6) | worker (generated, failed, stopped_budget), the detect job's terminal transaction (queued to failed or stopped_budget when detection does not finish), reviewer edit, attribute fix, applied regeneration, start-up re-check (D7) | not ended | US-00-003, US-00-004, US-00-006 to US-00-010 | no | UNDEFINED |
| rule result | listings | worker after generation; API after an edit; start-up re-check | none | replaced when the listing gets a new version | US-00-004, US-00-007 | no | current version only |
| approval | listings | reviewer bulk approve | none | not ended (audit trail) | US-00-009, US-00-010 | no | UNDEFINED |
| regeneration request | listings | reviewer regenerate | worker (applied, superseded, failed, stopped_budget) | not ended | US-00-008 | no | UNDEFINED |
| job | platform | generation start, resume, regenerate, re-run | worker claim (claim_token), retry, finish; stale-claim sweep; gateway refusal; a restarted run cancels its queued jobs | purge after finish, period UNDEFINED | US-00-002, US-00-003, US-00-008, US-00-012 | no | UNDEFINED |
| AI call | cost | gateway reservation before each call | gateway after the call; reservation sweep | not ended (the spend ledger) | US-00-011, US-00-012 | no | UNDEFINED |
| budget | cost | schema.sql (one row) | gateway (blocked_at), operator (limit, clearing the block) | never | US-00-012 | no | permanent |
| export | export | reviewer export | none | not ended | US-00-010 | no | UNDEFINED |
| config re-check | listings | API start-up when a channel's config hash changed (D7) | none | not ended | US-00-004, US-00-005 | no | UNDEFINED |
| export file | export | reviewer export, one per channel with approved rows | none | file purge, period UNDEFINED | US-00-010 | no | UNDEFINED |

Writer sequences the constraints rely on:

- CSV upload: insert the `uploads` row with counts 0, 0, 0; insert each row's product inside a savepoint with `ON CONFLICT (lower(sku)) DO NOTHING`, so a SKU taken by a concurrent upload becomes a row error instead of aborting the file (D21, AC-US-00-001-2); then update the three counts in the same transaction, which `chk_uploads_counts` checks.
- Detection end: the detect job's terminal transaction sets `detection_status` and, when detection did not finish (failed or stopped_budget), moves the product's `queued` listings to `failed` (with the reason) or `stopped_budget`; D6's re-run re-queues detection for products whose `detection_status` is `failed` or `stopped_budget`, and generation for their listings.
- Generate and regenerate results: the result transaction locks the product's attributes row and applies only if `revision` still equals the job's `attributes_revision`; otherwise it queues the work again with the new revision. Every finishing update matches `id`, `status = 'running'` and `claim_token`, so a job whose lease was swept cannot finish twice.
- Regenerate request: allowed only on a listing with `status = 'generated'`; the API refuses others with a message, so `base_value` is never null.
- Start-up re-check (D7): for each channel whose config hash changed, re-validate its listings; a listing that goes from passing to failing gets `version + 1`, which clears its approval (tenet 5); one `config_rechecks` row records the counts.
- Erasure of a person: `users.email` becomes `erased-<id>@invalid` and `password_hash` a value no password matches; the row stays because approvals name it.

Not modelled: channel rules (configuration files, REQ-017); unmatched image files (AC-US-00-001-4 lists them in the upload response only, no later read); listing edit history (HLD section 4 named it, but no story reads it; the analytics question in HLD section 11 is not a story); the 30 labelled eval images (files beside `make eval-detect`, D22).

## 3. Relationships

| From | To | Cardinality | FK column | On delete | Why |
| --- | --- | --- | --- | --- | --- |
| users | sessions | one-to-many | sessions.user_id | CASCADE | Deleting a user ends their sessions; a session without its user is meaningless. |
| users | uploads | one-to-many | uploads.uploaded_by | RESTRICT | A user who uploaded cannot be deleted silently; uploads name who loaded the launch. |
| uploads | upload_row_errors | one-to-many | upload_row_errors.upload_id | CASCADE | Row errors exist only to explain their upload and go with it. |
| brands | products | one-to-many | products.brand_id | RESTRICT | A brand with products cannot be deleted; the voice note and listings depend on it. |
| uploads | products | one-to-many | products.upload_id | RESTRICT | An upload with products cannot be deleted; images attach through it. |
| products | product_images | one-to-many | product_images.product_id | RESTRICT | No story deletes products; an image never outlives its product unnoticed. |
| products | product_attributes | one-to-zero-or-one | product_attributes.product_id | RESTRICT | One attribute row per product; no story deletes either. |
| users | product_attributes | many-to-zero-or-one | product_attributes.corrected_by | RESTRICT | A reviewer who corrected attributes stays named; erase their email instead of deleting (open concern). |
| users | generation_runs | one-to-many | generation_runs.started_by | RESTRICT | Runs name who started them. |
| products | listings | one-to-many | listings.product_id | RESTRICT | A product with listings cannot be deleted; listings are the product's output. |
| listings | rule_results | one-to-many | rule_results.listing_id | CASCADE | Rule results are derived from their listing and go with it. |
| listings | approvals | one-to-many | approvals.listing_id | RESTRICT | Approvals are the audit trail of REQ-015 and must not vanish with a listing. |
| users | approvals | one-to-many | approvals.approved_by | RESTRICT | An approval keeps naming its reviewer (ADR-0006). |
| listings | regeneration_requests | one-to-many | regeneration_requests.listing_id | RESTRICT | A request keeps its listing; no story deletes listings. |
| users | regeneration_requests | one-to-many | regeneration_requests.requested_by | RESTRICT | A request names the reviewer who asked. |
| generation_runs | jobs | many-to-zero-or-one | jobs.run_id | RESTRICT | A run with jobs cannot be deleted; progress is read through it. |
| products | jobs | one-to-many | jobs.product_id | RESTRICT | Jobs refer to live products; no story deletes products. |
| listings | jobs | many-to-zero-or-one | jobs.listing_id | RESTRICT | Jobs refer to live listings; no story deletes listings. |
| regeneration_requests | jobs | many-to-zero-or-one | jobs.regeneration_request_id | RESTRICT | A job serves its request; the request is kept. |
| jobs | ai_calls | many-to-zero-or-one | ai_calls.job_id | SET NULL | The cost ledger outlives purged jobs; the link is cleared, the cost stays. |
| products | ai_calls | many-to-zero-or-one | ai_calls.product_id | RESTRICT | A product's cost history must not vanish (AC-US-00-011-3). |
| users | exports | one-to-many | exports.created_by | RESTRICT | An export names the reviewer who made it. |
| exports | export_files | one-to-many | export_files.export_id | CASCADE | Files belong to their export and go with it. |

The mermaid diagram and one sentence per relationship are in `docs/design/erd.md`. Foreign keys cross area lines (listings to products, jobs to listings): a foreign key is a database guarantee, not a cross-area query, so tenet 1 holds; splitting an area into its own service later means dropping these keys (section 11).

## 4. PostgreSQL tables

Tables appear in the order `schema.sql` creates them. Ids are `bigint` identity (deviation 1, section 9); every list query is small enough that the deliberate sequential scans in section 9 are named rather than indexed.

### `users`: Users (hot: no)

A seeded person who signs in as a seller or a reviewer. Area: auth.

Serves ADR-0006, US-00-009, US-00-010. Expected volume: a handful of rows (10^1), one per seeded person; no self sign-up (ADR-0006).

| Column | Type | Null | Key | Default | Description | Why |
| --- | --- | --- | --- | --- | --- | --- |
| `id` | `bigint` | No | PK | `identity` | Surrogate key; referenced by sessions, uploads, runs, approvals and requests. | ADR-0006: approvals record the reviewer who made them. |
| `email` | `text` | No | UK |  | Sign-in address, unique ignoring case. **(personal data)** | ADR-0006: seeded accounts sign in with email and password. |
| `role` | `user_role` | No |  |  | seller or reviewer; checked by the API on every route. | ADR-0006 and AC-US-00-009-5: only reviewers approve, edit, regenerate and export. |
| `password_hash` | `text` | No |  |  | Password hash (argon2id or bcrypt, chosen in the LLD), never the password. **(personal data)** | ADR-0006: passwords stored hashed. |
| `created_at` | `timestamptz` | No |  | `now()` | When the row was written. | Rule: audit columns (database/references/postgres.md). |
| `updated_at` | `timestamptz` | No |  | `now()` | Last change to this row, set by the repository on every update. | Rule: audit columns, set by the repository on every UPDATE. |

**Indexes**

- `uq_users_email`: unique on (lower(email)). Login looks a user up by email ignoring case; two accounts for one address are refused.

**Constraints**

- `chk_users_email_shape`: refuses an address the login form could never match, or one longer than 254 characters.

### `sessions`: Sessions (hot: no)

A signed-in browser session, carried by a secure HTTP-only cookie. Area: auth.

Serves ADR-0006. Expected volume: tens of rows (10^1), one per active browser, purged after expiry.

| Column | Type | Null | Key | Default | Description | Why |
| --- | --- | --- | --- | --- | --- | --- |
| `token_hash` | `text` | No | PK |  | SHA-256 of the session cookie token; the token itself is never stored. | ADR-0006: server-side sessions; a database leak must not hand out live sessions. |
| `user_id` | `bigint` | No | FK users.id |  | The signed-in user. | ADR-0006: every route checks the session's role. |
| `csrf_token_hash` | `text` | No |  |  | Hash of the CSRF token every state-changing request must send. | ADR-0006 consequences: CSRF protection on every state-changing request. |
| `expires_at` | `timestamptz` | No |  |  | When the session stops being accepted. | ADR-0006: sessions expire; checked inside every request, purged by a daily sweep. |
| `created_at` | `timestamptz` | No |  | `now()` | When the row was written. | Rule: audit columns (database/references/postgres.md). |

**Indexes**

- `idx_sessions_user_id`: on (user_id). FK index: a user's sessions, and no scan when a user row is deleted.
- `idx_sessions_expires_at`: on (expires_at). The daily sweep deletes sessions WHERE expires_at < now().

**Constraints**

- `chk_sessions_expiry_after_creation`: refuses a session that is already expired when written.

### `brands`: Brands (hot: no)

A brand named in the upload CSV, with its voice note. Area: catalogue.

Serves US-00-001, US-00-003. Expected volume: a few rows (10^0 to 10^1), one per brand in the CSV.

| Column | Type | Null | Key | Default | Description | Why |
| --- | --- | --- | --- | --- | --- | --- |
| `id` | `bigint` | No | PK | `identity` | Surrogate key; referenced by products. | REQ-001: every CSV row carries a brand. |
| `name` | `text` | No | UK |  | Brand name as written in the CSV, unique ignoring case. | REQ-001: the CSV brand column; an upload matches rows to an existing brand by name. |
| `voice_note` | `text` | Yes |  |  | Short note on the brand's voice; null until the seller writes one. | AC-US-00-003-4: the voice note goes in every generation prompt; AC-US-00-003-5: null means ask before generating. |
| `created_at` | `timestamptz` | No |  | `now()` | When the row was written. | Rule: audit columns (database/references/postgres.md). |
| `updated_at` | `timestamptz` | No |  | `now()` | Last change to this row, set by the repository on every update. | Rule: audit columns, set by the repository on every UPDATE. |

**Indexes**

- `uq_brands_name`: unique on (lower(btrim(name))). An upload finds the brand for a CSV row by name ignoring case and outer spaces.

**Constraints**

- `chk_brands_name_length`: refuses a blank brand name or one longer than 100 characters.
- `chk_brands_voice_note_length`: refuses an empty-string voice note (use null) or one too long to fit a prompt.

### `uploads`: Uploads (hot: no)

One CSV upload by a seller, with its row counts. Area: catalogue.

Serves US-00-001. Expected volume: a few rows per launch (10^1 in total).

| Column | Type | Null | Key | Default | Description | Why |
| --- | --- | --- | --- | --- | --- | --- |
| `id` | `bigint` | No | PK | `identity` | Surrogate key; the upload images are attached to. | HLD section 5: /v1/uploads/{id}/images attaches images to this upload. |
| `uploaded_by` | `bigint` | No | FK users.id |  | The seller who uploaded. | ADR-0006: only signed-in users upload; the upload names who. |
| `file_name` | `text` | No |  |  | Name of the uploaded CSV file. | AC-US-00-001-1: the upload summary names the file. |
| `rows_total` | `integer` | No |  |  | Data rows in the file. | AC-US-00-001-1: the summary shows the count. |
| `rows_accepted` | `integer` | No |  |  | Rows that became products. | AC-US-00-001-1: one product per valid row. |
| `rows_rejected` | `integer` | No |  |  | Rows refused, each listed in upload_row_errors. | AC-US-00-001-2: rejected rows are reported; the rest still load. |
| `created_at` | `timestamptz` | No |  | `now()` | When the row was written. | HLD section 11: B1 measures time from upload to export. |
| `updated_at` | `timestamptz` | No |  | `now()` | Last change to this row, set by the repository on every update. | Rule: audit columns, set by the repository on every UPDATE. |

**Indexes**

- `idx_uploads_uploaded_by`: on (uploaded_by). FK index: no scan when a user row is checked on delete.

**Constraints**

- `chk_uploads_counts`: refuses a summary whose accepted and rejected rows do not add up to the file.
- `chk_uploads_file_name_length`: refuses an empty or overlong file name.

### `upload_row_errors`: Upload row errors (hot: no)

A CSV row the upload refused, with its reason. Area: catalogue.

Serves US-00-001. Expected volume: tens of rows per upload at most (10^1 to 10^2).

| Column | Type | Null | Key | Default | Description | Why |
| --- | --- | --- | --- | --- | --- | --- |
| `id` | `bigint` | No | PK | `identity` | Surrogate key. | AC-US-00-001-2: each rejected row is one line in the summary. |
| `upload_id` | `bigint` | No | FK uploads.id |  | The upload the row came from. | AC-US-00-001-2: errors are shown per upload. |
| `row_number` | `integer` | No |  |  | Line number in the CSV file, the header being line 1. | AC-US-00-001-2: the reason is shown with its row number. |
| `sku` | `text` | Yes |  |  | SKU on the refused row; null when the row had none. | AC-US-00-001-2 and D21: a missing or already-existing SKU is a rejection reason. |
| `reason` | `text` | No |  |  | Why the row was refused, as shown to the seller. | AC-US-00-001-2: the row is rejected with its reason. |
| `created_at` | `timestamptz` | No |  | `now()` | When the row was written. | Rule: audit columns (database/references/postgres.md). |

**Indexes**

- `idx_upload_row_errors_upload_id`: on (upload_id). FK index and the upload summary: an upload's rejected rows.

**Constraints**

- `chk_upload_row_errors_row_number`: refuses an error pointing at the header line or before it.
- `chk_upload_row_errors_reason_length`: refuses an empty or overlong reason.

### `products`: Products (hot: no)

One SKU from the CSV: the thing listings describe. Area: catalogue.

Serves US-00-001, US-00-002, US-00-003, US-00-011. Expected volume: 300 rows per launch, about 600 for the budget's life (10^2 to 10^3), driven by SKUs per launch.

| Column | Type | Null | Key | Default | Description | Why |
| --- | --- | --- | --- | --- | --- | --- |
| `id` | `bigint` | No | PK | `identity` | Surrogate key; referenced by images, attributes, listings, jobs and AI calls. | REQ-001: one product per CSV row. |
| `sku` | `text` | No | UK |  | Seller's product code, unique ignoring case across Catalift. | REQ-001 and D21: a SKU already in Catalift is rejected; D20 matches image names to it. |
| `brand_id` | `bigint` | No | FK brands.id |  | The product's brand. | REQ-001: the CSV brand column; AC-US-00-003-4 picks the voice note by brand. |
| `upload_id` | `bigint` | No | FK uploads.id |  | The upload that created the product. | AC-US-00-001-3: images uploaded to an upload attach to that upload's SKUs. |
| `category` | `text` | No |  |  | Category from the CSV. | REQ-001: the CSV category column. |
| `price_minor` | `bigint` | No |  |  | Price in minor units of currency (paise). | REQ-001: the CSV price column; Rule: money in minor units. |
| `currency` | `char(3)` | No |  | `'INR'` | ISO 4217 code of price_minor. | Rule: money carries its currency; the CSV has none, so INR is assumed (open concern). |
| `created_at` | `timestamptz` | No |  | `now()` | When the row was written. | Rule: audit columns (database/references/postgres.md). |
| `updated_at` | `timestamptz` | No |  | `now()` | Last change to this row, set by the repository on every update. | Rule: audit columns, set by the repository on every UPDATE. |

**Indexes**

- `uq_products_sku`: unique on (lower(sku)). D21: an upload rejects a SKU already present; D20 image matching looks SKUs up.
- `idx_products_brand_id`: on (brand_id). FK index; generation reads a brand's products.
- `idx_products_upload_id`: on (upload_id). FK index; image upload lists an upload's SKUs.

**Constraints**

- `chk_products_sku_shape`: refuses a SKU with spaces, or empty, or over 64 characters, which image names could not carry.
- `chk_products_category_length`: refuses a blank or overlong category.
- `chk_products_price_positive`: refuses a zero or negative price, which no listing can show.
- `chk_products_currency_iso`: refuses a lowercase or partial currency code.

### `product_images`: Product images (hot: no)

A photo attached to a product, with its 1024 px detection copy. Area: catalogue.

Serves US-00-001, US-00-002. Expected volume: about 2 per product (10^3), driven by photos per SKU.

| Column | Type | Null | Key | Default | Description | Why |
| --- | --- | --- | --- | --- | --- | --- |
| `id` | `bigint` | No | PK | `identity` | Surrogate key. | AC-US-00-001-3: each matched image is attached to one product. |
| `product_id` | `bigint` | No | FK products.id |  | The product the image belongs to. | AC-US-00-001-3 and D20: the image is attached to the SKU its name matches. |
| `position` | `smallint` | No |  |  | Order among the product's images; 1 drives detection. | Q-005: several images per SKU, the first drives detection. |
| `original_file_name` | `text` | No |  |  | File name as uploaded. | D20: the match rule runs on this name; the summary shows it. |
| `original_path` | `text` | No |  |  | Where the original is stored on the data disk, under a generated name. | HLD section 9: images are stored under generated names, never the uploaded path. |
| `detection_path` | `text` | No |  |  | Where the copy scaled to at most 1024 px on its long side is stored. | D23: detection sends only the 1024 px copy. |
| `content_type` | `text` | No |  |  | Sniffed image type. | HLD section 9: images are type-sniffed at upload. |
| `byte_size` | `integer` | No |  |  | Size of the original in bytes. | HLD section 9: uploads are capped at 10 MB per image. |
| `created_at` | `timestamptz` | No |  | `now()` | When the row was written. | Rule: audit columns (database/references/postgres.md). |

**Indexes**

- `uq_product_images_product_position`: unique on (product_id, position). FK index, and detection reads the product's image at position 1; two images cannot both be first.

**Constraints**

- `chk_product_images_position`: refuses a position that cannot be first or later.
- `chk_product_images_content_type`: refuses a file that is not an image the model accepts.
- `chk_product_images_byte_size`: refuses an empty file or one over the 10 MB cap.
- `chk_product_images_path_length`: refuses an empty or overlong name or path.

### `product_attributes`: Product attributes (hot: no)

The five detected attributes of a product, owned by listings (D5). Area: listings.

Serves US-00-002, US-00-007. Expected volume: one row per product (10^2 to 10^3).

| Column | Type | Null | Key | Default | Description | Why |
| --- | --- | --- | --- | --- | --- | --- |
| `product_id` | `bigint` | No | PK, FK products.id |  | The product described; one row each. | REQ-003: attributes are detected per product; D5 puts them in listings. |
| `detection_status` | `detection_status` | No |  | `'pending'` | pending, done, failed or stopped_budget. | AC-US-00-002-2: each product shows detection done or failed; AC-US-00-012-2: work refused by the budget shows stopped. |
| `revision` | `integer` | No |  | `0` | Raised by every detection result and every reviewer correction. | D5 and review fix: text generated from an older attribute revision is not applied. |
| `colour` | `text` | Yes |  |  | Detected or corrected colour; 'unknown' when unclear; null until done. | REQ-003 and AC-US-00-002-3: unclear attributes are stored as 'unknown'. |
| `pattern` | `text` | Yes |  |  | Detected or corrected pattern; null until done. | REQ-003. |
| `sleeve` | `text` | Yes |  |  | Detected or corrected sleeve; null until done. | REQ-003. |
| `neckline` | `text` | Yes |  |  | Detected or corrected neckline; null until done. | REQ-003. |
| `fit` | `text` | Yes |  |  | Detected or corrected fit; null until done. | REQ-003. |
| `detection_error` | `text` | Yes |  |  | Why detection failed; null unless failed. | AC-US-00-002-2: a failed product shows the reason. |
| `corrected_by` | `bigint` | Yes | FK users.id |  | Reviewer who last corrected an attribute; null while only detected. | AC-US-00-007-3 and D5: a reviewer can correct a detected attribute. |
| `created_at` | `timestamptz` | No |  | `now()` | When the row was written. | Rule: audit columns (database/references/postgres.md). |
| `updated_at` | `timestamptz` | No |  | `now()` | Last change to this row, set by the repository on every update. | Rule: audit columns, set by the repository on every UPDATE. |

**Indexes**

- `idx_product_attributes_corrected_by`: on (corrected_by). FK index: no scan when a user row is checked on delete.

**Constraints**

- `chk_product_attributes_done_complete`: refuses a product marked done with an attribute missing (AC-US-00-002-1).
- `chk_product_attributes_error_iff_failed`: refuses a failed product without a reason, or a reason on a product that did not fail.
- `chk_product_attributes_revision`: refuses a negative revision.
- `chk_product_attributes_value_length`: refuses an empty-string or overlong attribute value.

### `generation_runs`: Generation runs (hot: no)

One start of generation by a seller. Area: listings.

Serves US-00-003, US-00-012. Expected volume: a few per launch (10^1).

| Column | Type | Null | Key | Default | Description | Why |
| --- | --- | --- | --- | --- | --- | --- |
| `id` | `bigint` | No | PK | `identity` | Surrogate key; jobs and the progress view group by it. | AC-US-00-003-6: the seller sees per-product progress for a run. |
| `started_by` | `bigint` | No | FK users.id |  | Who started the run. | ADR-0006: every action names the signed-in user. |
| `neutral_voice_confirmed` | `boolean` | No |  | `false` | True when the seller confirmed a neutral voice for brands with no voice note. | AC-US-00-003-5: with no voice note, the seller confirms a neutral default before generation starts. |
| `created_at` | `timestamptz` | No |  | `now()` | When the row was written. | Rule: audit columns (database/references/postgres.md). |
| `updated_at` | `timestamptz` | No |  | `now()` | Last change to this row, set by the repository on every update. | Rule: audit columns, set by the repository on every UPDATE. |

**Indexes**

- `idx_generation_runs_started_by`: on (started_by). FK index: no scan when a user row is checked on delete.

### `listings`: Listings (hot: no)

One product on one channel: its text, version and rule status. Area: listings.

Serves US-00-003, US-00-004, US-00-006, US-00-007, US-00-008, US-00-009, US-00-010. Expected volume: one per product and enabled channel: 600 per launch, about 1,200 to 1,800 for the budget's life (10^3).

| Column | Type | Null | Key | Default | Description | Why |
| --- | --- | --- | --- | --- | --- | --- |
| `id` | `bigint` | No | PK | `identity` | Surrogate key; the grid row and the URL of a listing. | AC-US-00-007-1: each grid row is one product and channel. |
| `product_id` | `bigint` | No | FK products.id |  | The product described. | REQ-004: a listing per product per enabled channel. |
| `channel` | `text` | No |  |  | Channel id from configuration, such as amazon_style. | REQ-017: channels are configuration, so a new channel needs no migration (text, not an enum). |
| `status` | `listing_status` | No |  | `'queued'` | queued, generated, failed or stopped_budget. | D6: re-runs pick up each product and channel that is missing, failed or stopped by the budget. |
| `failure_reason` | `text` | Yes |  |  | Why generation failed; null unless failed. | HLD section 7: the product shows 'failed' with its reason. |
| `version` | `integer` | No |  | `0` | 0 before generation; raised by every change to the text or the product's attributes. | Tenet 5, D5, D7, D12: any change clears approval; approvals match on version. |
| `title` | `text` | Yes |  |  | Generated or edited title; null until generated. | REQ-004. |
| `bullet_1` | `text` | Yes |  |  | Bullet point 1; null until generated. | REQ-005: exactly 5 bullets (AC-US-00-003-2). |
| `bullet_2` | `text` | Yes |  |  | Bullet point 2; null until generated. | REQ-005. |
| `bullet_3` | `text` | Yes |  |  | Bullet point 3; null until generated. | REQ-005. |
| `bullet_4` | `text` | Yes |  |  | Bullet point 4; null until generated. | REQ-005. |
| `bullet_5` | `text` | Yes |  |  | Bullet point 5; null until generated. | REQ-005. |
| `description` | `text` | Yes |  |  | Generated or edited description; null until generated. | REQ-006. |
| `rule_status` | `rule_status` | No |  | `'unchecked'` | unchecked, passing or failing for the current version. | AC-US-00-004-1: every listing shows passing or failing; AC-US-00-009-3: approval needs passing. |
| `rule_config_hash` | `text` | Yes |  |  | Hash of the channel config the current status was checked against. | D7: at start-up, listings whose hash differs are re-checked. |
| `first_pass_passed` | `boolean` | Yes |  |  | Whether the first generated version passed the rules; set once. | D19: B2 is the share passing at first generation. |
| `first_pass_failed_rules` | `text[]` | Yes |  |  | Rule names the first version failed; set once with first_pass_passed. | D19: the failing rule names are kept with the first-pass result. |
| `created_at` | `timestamptz` | No |  | `now()` | When the row was written. | Rule: audit columns (database/references/postgres.md). |
| `updated_at` | `timestamptz` | No |  | `now()` | Last change to this row, set by the repository on every update. | Rule: audit columns, set by the repository on every UPDATE. |

**Indexes**

- `uq_listings_product_channel`: unique on (product_id, channel). FK index; one listing per product and channel; re-runs (D6) find the pair to re-queue.

**Constraints**

- `chk_listings_channel_shape`: refuses a channel id the configuration loader could never produce.
- `chk_listings_version`: refuses a negative version.
- `chk_listings_generated_complete`: refuses a generated listing missing its title, a bullet or the description (AC-US-00-003-1 to -3).
- `chk_listings_failure_reason`: refuses a failed listing without a reason, or a reason on one that did not fail.
- `chk_listings_text_length`: refuses empty-string or runaway text; channel limits stay in the rules engine.
- `chk_listings_first_pass_pair`: refuses a first-pass result without its rule list, a pass with failed rules, or a fail with none.
- `chk_listings_generated_checked`: refuses a generated listing shown without a rule result (AC-US-00-004-1).

### `rule_results`: Rule results (hot: no)

One rule failure of one listing version. Area: listings.

Serves US-00-004, US-00-007. Expected volume: a few failures per failing listing; older versions purged (10^2 to 10^3).

| Column | Type | Null | Key | Default | Description | Why |
| --- | --- | --- | --- | --- | --- | --- |
| `id` | `bigint` | No | PK | `identity` | Surrogate key. | REQ-009: each failure is shown on its own. |
| `listing_id` | `bigint` | No | FK listings.id |  | The listing that failed. | AC-US-00-004-5: a failure shows against its channel's listing only. |
| `listing_version` | `integer` | No |  |  | Version the rule was checked against. | AC-US-00-004-6: a changed listing is checked again; older results are purged. |
| `rule` | `text` | No |  |  | Rule name, such as title_max_length. | AC-US-00-004-2 to -4: the failure names the rule. |
| `field` | `text` | No |  |  | Field the failure is in, such as title or bullet_3. | AC-US-00-004-3: a banned word names its field. |
| `message` | `text` | No |  |  | Detail shown to the reviewer: limit and length, the word, or the missing attribute. | AC-US-00-004-2 to -4: the failure states what is wrong. |
| `config_hash` | `text` | No |  |  | Hash of the channel config used. | D7: results are tied to the rules they were checked against. |
| `created_at` | `timestamptz` | No |  | `now()` | When the row was written. | Rule: audit columns (database/references/postgres.md). |

**Indexes**

- `idx_rule_results_listing_version`: on (listing_id, listing_version). FK index; the grid and the approval check read a listing's failures for its current version.

**Constraints**

- `chk_rule_results_text_length`: refuses an empty or overlong rule, field or message.
- `chk_rule_results_version`: refuses a result for a listing that was never generated.

### `approvals`: Approvals (hot: no)

A reviewer's approval of one listing version. Area: listings.

Serves US-00-009, US-00-010. Expected volume: about one per listing version approved (10^3).

| Column | Type | Null | Key | Default | Description | Why |
| --- | --- | --- | --- | --- | --- | --- |
| `id` | `bigint` | No | PK | `identity` | Surrogate key. | REQ-012: each approval is recorded. |
| `listing_id` | `bigint` | No | FK listings.id |  | The listing approved. | REQ-015: only approved listings are exported. |
| `listing_version` | `integer` | No |  |  | Version approved; the listing counts as approved only while its version still equals this. | Tenet 5 and Q-022: any change clears approval, so approval is matched on version, not stored on the listing. |
| `approved_by` | `bigint` | No | FK users.id |  | The reviewer who approved. | ADR-0006: approvals record the reviewer who made them. |
| `created_at` | `timestamptz` | No |  | `now()` | When the row was written. | AC-US-00-009-4: an approved listing shows when it was approved. |

**Indexes**

- `uq_approvals_listing_version`: unique on (listing_id, listing_version). FK index; export and the grid join approvals on (listing_id, version); a version is approved once.
- `idx_approvals_approved_by`: on (approved_by). FK index: no scan when a user row is checked on delete.

**Constraints**

- `chk_approvals_version`: refuses an approval of a listing that was never generated.

### `regeneration_requests`: Regeneration requests (hot: no)

A reviewer's request to rewrite one field with an instruction. Area: listings.

Serves US-00-008. Expected volume: a few per listing at most (10^2 to 10^3).

| Column | Type | Null | Key | Default | Description | Why |
| --- | --- | --- | --- | --- | --- | --- |
| `id` | `bigint` | No | PK | `identity` | Surrogate key; the request id in the regenerate job key. | D12: regenerate jobs are keyed by listing, field and request id. |
| `listing_id` | `bigint` | No | FK listings.id |  | The listing to change. | REQ-013: regenerate a field of a listing. |
| `field` | `text` | No |  |  | title, bullet_1 to bullet_5, or description. | AC-US-00-008-1 and -2: only that field changes. |
| `instruction` | `text` | No |  |  | Reviewer's free-text instruction. | REQ-013 and AC-US-00-008-3: the instruction goes in the prompt. |
| `base_value` | `text` | No |  |  | The field's value when the request was made. | D12: the result applies only if the field still holds this value. |
| `status` | `regeneration_status` | No |  | `'queued'` | queued, applied, superseded, failed or stopped_budget. | D12: a dropped result shows 'superseded by your edit'; D13: refused requests end stopped_budget. |
| `requested_by` | `bigint` | No | FK users.id |  | The reviewer who asked. | ADR-0006: only reviewers regenerate. |
| `created_at` | `timestamptz` | No |  | `now()` | When the row was written. | Rule: audit columns (database/references/postgres.md). |
| `updated_at` | `timestamptz` | No |  | `now()` | Last change to this row, set by the repository on every update. | Rule: audit columns, set by the repository on every UPDATE. |

**Indexes**

- `idx_regeneration_requests_listing_id`: on (listing_id). FK index; the grid shows a listing's open requests.
- `idx_regeneration_requests_requested_by`: on (requested_by). FK index: no scan when a user row is checked on delete.

**Constraints**

- `chk_regeneration_requests_field`: refuses a request for a field a listing does not have.
- `chk_regeneration_requests_instruction_length`: refuses a blank or overlong instruction.

### `jobs`: Jobs (hot: yes, during a run (about 1 write a second))

One unit of background AI work, claimed by the worker (ADR-0005). Area: platform.

Serves US-00-002, US-00-003, US-00-008, US-00-012. Expected volume: about 900 per launch (10^3); written about once a second during a run.

| Column | Type | Null | Key | Default | Description | Why |
| --- | --- | --- | --- | --- | --- | --- |
| `id` | `bigint` | No | PK | `identity` | Surrogate key; logs carry it. | HLD section 10: every job log line carries the job id. |
| `type` | `job_type` | No |  |  | detect_attributes, generate_listing or regenerate_field. | HLD section 5: the three job types. |
| `dedupe_key` | `text` | No |  |  | Product, step and channel, or listing, field and request id for regenerations. | ADR-0005 check 6 and D12: a job delivered twice does nothing twice. |
| `status` | `job_status` | No |  | `'queued'` | queued, running, done, failed, stopped_budget or cancelled. | ADR-0005: retries and terminal failed; D13 stopped_budget; HLD section 7: a restarted run cancels its queued jobs. |
| `run_id` | `bigint` | Yes | FK generation_runs.id |  | The run the job belongs to; null for regenerations. | D13: regenerate jobs belong to no run. |
| `product_id` | `bigint` | No | FK products.id |  | The product the work is for. | AC-US-00-002-2 and AC-US-00-003-6: progress and failures per product. |
| `listing_id` | `bigint` | Yes | FK listings.id |  | The listing written; null for detection. | D6: generation work is per product and channel. |
| `regeneration_request_id` | `bigint` | Yes | FK regeneration_requests.id |  | The request served; null unless a regeneration. | D12: the job carries its request. |
| `attempts` | `smallint` | No |  | `0` | Attempts used, 0 to 3; a 429 does not use one. | D14: 3 attempts; D15: rate limits do not use an attempt. |
| `run_after` | `timestamptz` | No |  | `now()` | Earliest time the job may be claimed. | ADR-0005: retries wait in the table with a run-after time. |
| `first_attempt_at` | `timestamptz` | Yes |  |  | When the first attempt started; null until claimed. | D15: a job still rate limited 1 hour after its first try fails. |
| `attributes_revision` | `integer` | Yes |  |  | Attribute revision read when the prompt was built; null for detection. | Review fix for D5: the result applies only if product_attributes.revision is unchanged, otherwise the work is queued again. |
| `claim_token` | `uuid` | Yes |  |  | Random token set on every claim; null unless running. | Review fix: a late finisher whose lease was swept cannot finish a job someone else now holds. |
| `claimed_by` | `text` | Yes |  |  | Worker process holding the claim; null unless running. | ADR-0005: claims with row locking; the sweep returns stale claims. |
| `lease_until` | `timestamptz` | Yes |  |  | When the claim expires; null unless running. | HLD section 8: a 2 minute lease, longer than the 60 second call timeout. |
| `last_error` | `text` | Yes |  |  | Most recent error; null when none. | AC-US-00-002-2: a failure shows its reason. |
| `created_at` | `timestamptz` | No |  | `now()` | When the row was written. | Rule: audit columns (database/references/postgres.md). |
| `updated_at` | `timestamptz` | No |  | `now()` | Last change to this row, set by the repository on every update. | Rule: audit columns, set by the repository on every UPDATE. |
| `finished_at` | `timestamptz` | Yes |  |  | When the job reached done, failed, stopped_budget or cancelled. | HLD section 4: finished jobs are purged after a retention period (UNDEFINED). |

**Indexes**

- `idx_jobs_claimable`: on (run_after, id), partial WHERE status = 'queued'. The claim: SELECT ... WHERE status = 'queued' AND run_after <= now() ORDER BY run_after, id FOR UPDATE SKIP LOCKED.
- `idx_jobs_running_lease`: on (lease_until), partial WHERE status = 'running'. The stale-claim sweep: WHERE status = 'running' AND lease_until < now().
- `uq_jobs_dedupe_key_active`: unique on (dedupe_key), partial WHERE status IN ('queued', 'running'). ADR-0005 check 6: a second enqueue of live work is refused, while finished work can be queued again.
- `idx_jobs_run_id`: on (run_id). FK index; run progress and 'resume failed and stopped' (D6).
- `idx_jobs_product_id`: on (product_id). FK index; per-product progress.
- `idx_jobs_listing_id`: on (listing_id). FK index: no scan when a listing row is checked.
- `idx_jobs_regeneration_request_id`: on (regeneration_request_id). FK index: a request's job.

**Constraints**

- `chk_jobs_attempts`: refuses a fourth attempt, which D14 forbids.
- `chk_jobs_targets`: refuses a job missing the listing, request or attribute revision its type needs.
- `chk_jobs_claim`: refuses a running job with no lease, or a lease left on a job that is not running.
- `chk_jobs_finished`: refuses a terminal job with no finish time, or a finish time on live work.
- `chk_jobs_text_length`: refuses an empty or overlong key, error or worker id.

### `ai_calls`: AI calls (hot: no)

One call through the AI gateway: the cost ledger. Area: cost.

Serves US-00-011, US-00-012, US-00-003, US-00-008. Expected volume: about 900 per launch plus regenerations and evals (10^3).

| Column | Type | Null | Key | Default | Description | Why |
| --- | --- | --- | --- | --- | --- | --- |
| `id` | `bigint` | No | PK | `identity` | Surrogate key. | REQ-020: every AI call is recorded. |
| `job_id` | `bigint` | Yes | FK jobs.id |  | The job that made the call; null for evals or after the job is purged. | AC-US-00-011-1: the record names its purpose; the ledger outlives finished jobs. |
| `product_id` | `bigint` | Yes | FK products.id |  | The product the call was for; null only for eval calls. | AC-US-00-011-3: a product's cost is the sum of its calls. |
| `purpose` | `ai_call_purpose` | No |  |  | detect, generate, regenerate or eval_detect. | AC-US-00-011-1: the record holds the purpose; D22: eval calls count toward the block. |
| `model` | `text` | No |  |  | Model id, such as claude-haiku-4-5. | AC-US-00-011-1 and PRD Constraints. |
| `prompt_template_id` | `text` | No |  |  | Id of the prompt template rendered. | Required by AC-US-00-003-4: the prompt is traceable. |
| `prompt` | `text` | No |  |  | The rendered prompt sent. | AC-US-00-003-4 and AC-US-00-008-3: the voice note and the instruction are visible in the recorded prompt. |
| `status` | `ai_call_status` | No |  | `'reserved'` | reserved, succeeded, failed or possibly_charged. | HLD section 7: a reservation is written before the call and finalised after; a crash leaves possibly_charged. |
| `reserved_micro_usd` | `bigint` | No |  |  | Estimated cost reserved before the call, in millionths of a US dollar. | HLD section 3: the budget check reserves an estimate under the budget lock. |
| `cost_micro_usd` | `bigint` | Yes |  |  | Actual cost reported, in millionths of a US dollar; null until known. | REQ-020: cost of every call; AC-US-00-011-2: a failed call is recorded with its cost, if any. |
| `input_tokens` | `integer` | Yes |  |  | Input tokens reported; null until known. | REQ-020: token count of every call. |
| `output_tokens` | `integer` | Yes |  |  | Output tokens reported; null until known. | REQ-020. |
| `provider_generation_id` | `text` | Yes |  |  | OpenRouter's generation id; null when not returned. | TODOS.md reconciliation (D26) looks up real cost by this id. |
| `error` | `text` | Yes |  |  | Error returned; null on success. | AC-US-00-011-2: failed calls are still recorded. |
| `created_at` | `timestamptz` | No |  | `now()` | When the row was written. | Rule: audit columns (database/references/postgres.md). |
| `updated_at` | `timestamptz` | No |  | `now()` | Last change to this row, set by the repository on every update. | Rule: audit columns, set by the repository on every UPDATE. |

**Indexes**

- `idx_ai_calls_job_id`: on (job_id). FK index; SET NULL on job purge does not scan the ledger.
- `idx_ai_calls_product_id`: on (product_id). AC-US-00-011-3 and -4: cost per product.
- `idx_ai_calls_reserved`: on (created_at), partial WHERE status = 'reserved'. The sweep that turns a reservation left after the lease into possibly_charged.

**Constraints**

- `chk_ai_calls_amounts`: refuses a zero reservation or a negative cost or token count.
- `chk_ai_calls_succeeded_complete`: refuses a succeeded call with no cost or tokens (REQ-020).
- `chk_ai_calls_product_unless_eval`: refuses a product call without its product, which would hide it from the product's cost.
- `chk_ai_calls_text_length`: refuses an empty or runaway model, template id or prompt.

**How spend is counted (`ai_calls`, `budget`).** One expression, used by the gateway's check, `/v1/costs`, the banner and per-product cost alike: spend = SUM over `ai_calls` of `cost_micro_usd` for `succeeded`, `coalesce(cost_micro_usd, 0)` for `failed`, and `reserved_micro_usd` for `reserved` and `possibly_charged`. A provider error response before any generation ends `failed`; a timeout, a dropped connection or a worker crash between reservation and result ends `possibly_charged`. The gateway locks the `budget` row (`SELECT ... FOR UPDATE`), computes spend, refuses when spend plus the new reservation would pass `limit_micro_usd`, and sets `blocked_at` on its first refusal; while `blocked_at` is set it refuses every call, whatever its size, until the operator clears it (D13). The LLD ships the expression as one SQL view so the readers cannot drift.

### `budget`: Budget (hot: no)

The single row holding the AI spend limit and the blocked state. Area: cost.

Serves US-00-012. Expected volume: exactly one row (10^0).

| Column | Type | Null | Key | Default | Description | Why |
| --- | --- | --- | --- | --- | --- | --- |
| `id` | `smallint` | No | PK | `1` | Always 1; the row the gateway locks. | HLD section 7: the budget check and reservation happen under one lock (SELECT ... FOR UPDATE on this row). |
| `limit_micro_usd` | `bigint` | No |  | `8000000` | Spend after which calls are refused, in millionths of a US dollar. | REQ-021: calls are blocked once spending passes USD 8. |
| `blocked_at` | `timestamptz` | Yes |  |  | When the gateway first refused a call; null while not blocked. | D13: a stored budget-blocked state drives the banner until the owner clears it. |
| `created_at` | `timestamptz` | No |  | `now()` | When the row was written. | Rule: audit columns (database/references/postgres.md). |
| `updated_at` | `timestamptz` | No |  | `now()` | Last change to this row, set by the repository on every update. | Rule: audit columns, set by the repository on every UPDATE. |

**Constraints**

- `chk_budget_single_row`: refuses a second budget row the gateway would not lock.
- `chk_budget_limit_positive`: refuses a zero or negative limit.

### `exports`: Exports (hot: no)

One export run by a reviewer. Area: export.

Serves US-00-010. Expected volume: a few per launch (10^1).

| Column | Type | Null | Key | Default | Description | Why |
| --- | --- | --- | --- | --- | --- | --- |
| `id` | `bigint` | No | PK | `identity` | Surrogate key; the download URL names it. | HLD section 5: /v1/exports/{id}/files/{channel}. |
| `created_by` | `bigint` | No | FK users.id |  | The reviewer who exported. | ADR-0006: only reviewers export. |
| `created_at` | `timestamptz` | No |  | `now()` | When the row was written. | HLD section 11: B1 measures time from upload to export. |

**Indexes**

- `idx_exports_created_by`: on (created_by). FK index: no scan when a user row is checked on delete.

### `export_files`: Export files (hot: no)

One channel's CSV within an export. Area: export.

Serves US-00-010. Expected volume: one per channel per export (10^1 to 10^2).

| Column | Type | Null | Key | Default | Description | Why |
| --- | --- | --- | --- | --- | --- | --- |
| `id` | `bigint` | No | PK | `identity` | Surrogate key. | REQ-014: one CSV file per channel. |
| `export_id` | `bigint` | No | FK exports.id |  | The export the file belongs to. | AC-US-00-010-1: one export produces one file per channel. |
| `channel` | `text` | No |  |  | Channel id from configuration. | REQ-014: one file per channel. |
| `file_path` | `text` | No |  |  | Where the CSV is stored on the data disk. | HLD section 5: the file is downloaded later. |
| `row_count` | `integer` | No |  |  | Approved listings in the file. | AC-US-00-010-5: a channel with no approved rows gets no file. |
| `created_at` | `timestamptz` | No |  | `now()` | When the row was written. | Rule: audit columns (database/references/postgres.md). |

**Indexes**

- `uq_export_files_export_channel`: unique on (export_id, channel). FK index; one file per channel per export.

**Constraints**

- `chk_export_files_row_count`: refuses an empty file, which AC-US-00-010-5 says is never produced.
- `chk_export_files_channel_shape`: refuses a channel id the configuration loader could never produce.
- `chk_export_files_path_length`: refuses an empty or overlong path.

### `config_rechecks`: Config re-checks (hot: no)

One start-up re-check of listings after a channel's rules changed (D7). Area: listings.

Serves US-00-004, US-00-005. Expected volume: one row per channel per config change (10^0 to 10^1).

| Column | Type | Null | Key | Default | Description | Why |
| --- | --- | --- | --- | --- | --- | --- |
| `id` | `bigint` | No | PK | `identity` | Surrogate key. | D7: each re-check is recorded. |
| `channel` | `text` | No |  |  | Channel whose configuration changed. | D7: re-check runs per channel whose config hash differs. |
| `config_hash` | `text` | No |  |  | Hash of the configuration now loaded. | D7: listings are re-checked against this hash. |
| `listings_rechecked` | `integer` | No |  |  | Listings validated again. | D7: the count is logged and shown on the grid. |
| `approvals_cleared` | `integer` | No |  |  | Approved listings that now fail and lost their approval. | D7: approvals that now fail are cleared, with the count shown on the grid. |
| `created_at` | `timestamptz` | No |  | `now()` | When the row was written. | Rule: audit columns (database/references/postgres.md). |

**Indexes**

- `idx_config_rechecks_channel_created`: on (channel, created_at DESC). The grid shows the latest re-check per channel.

**Constraints**

- `chk_config_rechecks_counts`: refuses a cleared count larger than the listings checked.
- `chk_config_rechecks_channel_shape`: refuses a channel id or hash the loader could never produce.

## 5. Enumerations

| Name | Values | Why |
| --- | --- | --- |
| `user_role` | `seller`, `reviewer` | ADR-0006 names exactly two roles; a new role is an ADR and a migration. |
| `detection_status` | `pending`, `done`, `failed`, `stopped_budget` | AC-US-00-002-2: done or failed; AC-US-00-012-2: work refused by the budget shows stopped; pending before the worker finishes. |
| `listing_status` | `queued`, `generated`, `failed`, `stopped_budget` | D6 re-queues failed and stopped listings; D13 adds stopped_budget. |
| `rule_status` | `unchecked`, `passing`, `failing` | AC-US-00-004-1: passing or failing; unchecked only before the first check. |
| `regeneration_status` | `queued`, `applied`, `superseded`, `failed`, `stopped_budget` | D12 (superseded by your edit) and D13 (stopped by the budget). |
| `job_type` | `detect_attributes`, `generate_listing`, `regenerate_field` | HLD section 5 names the three job types; a new one is a code change anyway. |
| `job_status` | `queued`, `running`, `done`, `failed`, `stopped_budget`, `cancelled` | ADR-0005 lifecycle plus D13; cancelled when a restarted run replaces queued work (HLD section 7); rate-limited waits stay queued with a later run_after (D15). |
| `ai_call_purpose` | `detect`, `generate`, `regenerate`, `eval_detect` | AC-US-00-011-1 records the purpose; D22 adds eval calls that count toward the block. |
| `ai_call_status` | `reserved`, `succeeded`, `failed`, `possibly_charged` | HLD section 7: reserve, call, record; a crash or timeout leaves possibly_charged. |

Channels are deliberately not an enum: REQ-017 says a channel is added by configuration with no code change, and an enum value is a migration.

## 6. Retention and personal data

Rules marked **UNDEFINED** are decisions owed by a person before the first release; this document does not make them. One question covers most rows: how long after the demo is Catalift's data kept, and how is it destroyed? Owner: Karthik Reddy (product owner), by 2026-10-15. The HLD suggested values (sessions 7 days, finished jobs and export files 30 days); they are suggestions in that question, not rules here, and no purge job uses them.

| Table | Rule | Mechanism | Source |
| --- | --- | --- | --- |
| `users` | UNDEFINED (asked: keep accounts how long after the demo? erasure keeps the row and blanks email and password hash) | none until decided | not stated |
| `sessions` | Deleted once expires_at passes; session length UNDEFINED (asked; HLD suggested 7 days) | daily sweep using `idx_sessions_expires_at`; expiry also checked in every request | ADR-0006 (expiry), length not stated |
| `brands`, `uploads`, `products`, `product_images`, `product_attributes`, `generation_runs`, `listings`, `regeneration_requests`, `exports` | UNDEFINED (asked: the demo data question above) | none until decided | not stated |
| `upload_row_errors` | Lives as long as its upload | ON DELETE CASCADE from `uploads` | this design |
| `rule_results` | Only the current listing version's results are kept | the transaction that writes a version's results deletes the listing's older ones | HLD section 4 |
| `approvals` | UNDEFINED end; kept as the audit trail while the deployment lives (HLD section 4) | none until decided | HLD section 4 |
| `jobs` | Finished jobs purged after a period UNDEFINED (asked; HLD suggested 30 days) | none until decided; `ai_calls.job_id` is SET NULL so the ledger survives the purge | not stated |
| `ai_calls` | UNDEFINED end; kept as the spend ledger while the deployment lives (HLD section 4) | none until decided | HLD section 4 |
| `budget` | Permanent single row | `chk_budget_single_row` | REQ-021 |
| `config_rechecks` | UNDEFINED (the demo data question above) | none until decided | not stated |
| `export_files` | Rows follow their export; files on disk purged after a period UNDEFINED (asked; HLD suggested 30 days) | ON DELETE CASCADE from `exports`; no file purge until decided | not stated |

Personal-data columns (2): `users.email` (contact, sign-in identifier), `users.password_hash` (credential). Both have UNDEFINED retention. Prompts in `ai_calls.prompt` carry product data and the brand's voice note, not personal data.

## 7. Migration plan

| # | db-migration name | Phase (expand \| migrate \| contract) | Hot table | Lock risk and batch note |
| --- | --- | --- | --- | --- |
| 1 | initial_schema (schema.sql) | expand | no (empty database) | none; the file sets lock_timeout 2s and statement_timeout 60s as every migration does |

No repository exists yet, so there is no numbering convention to follow; the go-api kit uses goose, and migration 1 takes the next form that kit's repository defines when it is created. `jobs` becomes hot during a run (about one write a second): later changes to it follow the hot-table path (`NOT VALID` then `VALIDATE`, `CREATE INDEX CONCURRENTLY` under `-- +goose NO TRANSACTION`), and are deployed outside a run.

## 8. Design checks

- Every writer, every column: section 2 names each writer. `listings` has six writer paths (enqueue, worker, reviewer edit, attribute fix, regeneration, start-up re-check); every NOT NULL column has a default or comes from the creating path, and the text columns stay null until the worker generates them (`chk_listings_generated_complete`).
- Tenant-safe references: not applicable; one seller organisation per deployment (deviation 2).
- Predicates are immutable: no index or check calls now(). Expiring rows (`sessions.expires_at`, `jobs.lease_until`, reserved `ai_calls`) are handled by sweeps and by checks inside the writing transaction; a stale job claim can block its work for at most the 2 minute lease plus the 1 minute sweep interval (HLD section 8).
- Derived rows follow their parent: rule results are replaced per listing version and cascade with the listing; approvals match on version, so a new version leaves the old approval inert instead of deleting the audit row; a re-run (D6) re-queues a failed or stopped listing in place; text generated from an older attribute revision is never applied (review fix); detection that does not finish moves the product's queued listings with it.
- Lifetimes compose: nothing is deleted yet (retention UNDEFINED); `RESTRICT` everywhere a kept record points at a parent, `CASCADE` only for derived rows (row errors, rule results, export files, sessions).
- Size from the rule: every table stays under 10^4 rows within the USD 8 budget, so batched deletes suffice and no partitioning or BRIN is used.
- Ambiguous numbers read aloud: "3 attempts" (D14) is the first try plus two retries; `chk_jobs_attempts` allows 0 to 3. "Spend" is the single expression under `ai_calls` in section 4.
- Uniqueness has a lifecycle: SKU, email and brand name are unique ignoring case (`lower()`), globally (one organisation), and nothing is soft-deleted.

## 9. Rules and deviations

Rules checked: 18 against `database/references/postgres.md` (names, ids, timestamps, nullability, enums versus lookup tables, money, soft delete, foreign keys with ON DELETE and index, multi-tenant keys, text length checks, composite index order, partial index predicates, covering indexes, concurrent index builds, expand and contract, migration timeouts, JSONB, row-level security). Deviations: 5.

- deviation: ids, `bigint` identity instead of uuidv7 although listing and export ids appear in URLs; only signed-in users of one organisation ever see them, so a sequential id reveals nothing they cannot already list; revisit 2027-01-01 or when any URL becomes public.
- deviation: tenant_id first in composite keys, there is no tenant_id; one seller organisation per deployment (HLD section 9 assumption); revisit if a second organisation shares a deployment.
- deviation: money with a currency column, `ai_calls` and `budget` hold millionths of a US dollar in `bigint` with no currency column; OpenRouter bills only in USD and `numeric(19,4)` cannot hold sub-cent call costs; revisit if a second AI provider bills in another currency.
- deviation: both audit columns on every table, append-only tables (`sessions`, `upload_row_errors`, `product_images`, `rule_results`, `approvals`, `exports`, `export_files`) carry `created_at` only; no path updates them; revisit if any gains an update path.
- deviation: constraint and unique-index names, `chk_<table>_<rule>` and `uq_<table>_<cols>` (the data-model convention) instead of `<table>_<col>_check` and `_key`; revisit never, the gate expects these names.

Query shapes deliberately served by a sequential scan (4), each under 10^4 rows: the review grid list and its channel and rule-status filters over `listings` (AC-US-00-007-1 and -5); the spend sum over `ai_calls` under the budget lock; the start-up config-hash re-check over `listings` (D7); the B1 and B2 analytics queries (HLD section 11). Each gets an index if its table passes 10^4 rows.

## 10. What the review found

Reviewed by: critic, 2026-10-04, against US-00-001 to US-00-012, HLD v3 and ADR-0005, ADR-0006. The critic read the SQL by hand and found nothing PostgreSQL 16 would refuse; after the fixes, the file applies to an empty postgres:16 (section 12).

Findings: BLOCKER 0, MAJOR 4, MINOR 8, NIT 1 (open 4).

### MAJOR: detection that fails or is stopped by the budget strands the product's listings (`product_attributes`, `listings`)

Listings are created `queued` at generation start, but a detect job that ends failed or stopped left them queued forever, and the D6 re-run never selected them.

**Fix:** `detection_status` gains `stopped_budget`; the detect job's terminal transaction moves the product's queued listings to failed or stopped_budget; D6 re-queues detection for failed and stopped products (section 2, writer sequences). Status: fixed in this version.

### MAJOR: D7 "approvals that now fail are cleared" had no mechanism (`approvals`, `listings`)

Approval matches on version, and a config change did not change the version, so an approved listing that now fails would still export.

**Fix:** the start-up re-check raises `version` on a listing that goes from passing to failing, clearing its approval; `config_rechecks` stores the counts the grid shows (section 2, new table). Status: fixed in this version.

### MAJOR: an attribute correction during generation is overwritten by text written from the old attributes (`listings`, `product_attributes`)

A generate job built with "navy" could commit after the reviewer corrected to "black", and the presence-only rule would pass it.

**Fix:** `product_attributes.revision` and `jobs.attributes_revision`; results apply only when the revision is unchanged, otherwise the work is queued again (section 2). Status: fixed in this version.

### MAJOR: "spend" was an ambiguous number (`ai_calls`, `budget`)

The budget check, the banner and per-product cost could each sum differently, and the meaning of `blocked_at` as a gate was not stated.

**Fix:** one spend expression and the blocked rule are written under `ai_calls` in section 4; the LLD ships it as one view. Status: fixed in this version.

### MINOR: a late finisher cannot tell it lost its claim (`jobs`)

**Fix:** `jobs.claim_token`, required while running; every finishing update matches it. Status: fixed in this version.

### MINOR: no state for "a restarted run cancels its queued jobs" (`jobs`)

**Fix:** `job_status` gains `cancelled`, counted as finished by `chk_jobs_finished`. Status: fixed in this version.

### MINOR: the upload writer could not satisfy `chk_uploads_counts` in foreign-key order (`uploads`)

**Fix:** the writer sequence (counts 0, savepoint per row with `ON CONFLICT DO NOTHING`, counts updated in the same transaction) is stated in section 2. Status: fixed in this version.

### MINOR: the erasure path was refused by the schema (`users`)

**Fix:** erasure writes `erased-<id>@invalid` and an unmatchable hash, which the checks accept; section references corrected. Status: fixed in this version.

### MINOR: regenerating a null field failed with a constraint error (`regeneration_requests`)

**Fix:** the API allows regeneration only on a generated listing (section 2). Status: fixed in this version.

### MINOR: cross-row consistency between job targets and the cost ledger is not enforced (`jobs`, `ai_calls`)

A generate job could name a listing of another product, and `ai_calls.product_id` copied from it would charge the wrong product.

**Fix:** `UNIQUE (id, product_id)` on `listings`, `UNIQUE (id, listing_id)` on `regeneration_requests`, and composite foreign keys from `jobs`. Status: open (deferred to the LLD; the worker builds jobs from the rows it reads, so a mismatch needs a code bug first).

### MINOR: products can never be deleted, so any retention decision is blocked (`products`, `ai_calls`)

**Fix:** once the retention question in section 6 is answered, decide whether the ledger keeps `product_id` (SET NULL plus a copied SKU) and write the purge order. Status: open (waits on the retention answer).

### MINOR: schema.sql will not run unchanged as goose migration 1

**Fix:** when the repository exists, add the goose annotations, drop BEGIN and COMMIT, and use `SET LOCAL` for the timeouts. Status: open (no repository yet).

### NIT: wording and enforcement gaps

Fixed in this version: a pass with failed rules or a fail with none is refused (`chk_listings_first_pass_pair`); a generated listing must have a rule result (`chk_listings_generated_checked`); brand names are unique after trimming (`uq_brands_name` on `lower(btrim(name))`). Open: the HLD calls the reservation state `pending` while the schema says `reserved`; the HLD lists a field edit history this model drops (section 11); "set once" for the first-pass result is enforced by the writer, not the database. Status: open.

## 11. Open concerns

- **[gap]** The CSV has no currency, so `products.currency` defaults to INR. (`products`) Owner: Karthik Reddy (product owner), by 2026-10-15. Blocks development: no.
- **[ambiguity]** SKUs are unique ignoring case (`uq_products_sku` on `lower(sku)`); the reading taken is that D20 image matching also ignores case. (`products`, `product_images`) Owner: Karthik Reddy, by 2026-10-15. Blocks development: no.
- **[gap]** Retention is UNDEFINED for 17 of 19 tables, including both personal-data columns (section 6). Owner: Karthik Reddy (product owner), by 2026-10-15. Blocks development: no; blocks the first release.
- **[conflict]** HLD v3 section 4 lists a field edit history in the listings area; this model drops it because no story reads it (section 2). Either the HLD row goes or a story is added. Owner: Karthik Reddy, by 2026-10-15. Blocks development: no.
- **[conflict]** HLD v3 section 7 names the reservation state `pending`; the schema names it `reserved`. The HLD wording should follow. Owner: Karthik Reddy, by 2026-10-15. Blocks development: no.
- **[gap]** Replay mode's ledger rows are undecided (TODOS.md, D27): `ai_calls` has no replay marker yet, and the gateway LLD must add one or decide replay writes nothing, before phase 1. (`ai_calls`) Owner: Karthik Reddy, before phase 1. Blocks development: no.
- **[risk]** Foreign keys cross area lines (listings to products, jobs to listings); splitting an area into its own service later means dropping them. (`listings`, `jobs`, `ai_calls`) Owner: Karthik Reddy, no date (only if a split is planned). Blocks development: no.
- **[scope]** Composite foreign keys for job targets, the product purge order and the goose wrapper are open review findings (section 10). Owner: Karthik Reddy, at the LLD. Blocks development: no.

## 12. Applying this

`schema.sql` runs top to bottom in one transaction against an empty database: lock and statement timeouts, enum types, then tables in foreign-key order (`config_rechecks` last), then the single budget row. Use it as migration 1; every change after the first release is its own migration, never an edit to this file.

- Gate: `data-model: 19 tables, 161 columns, 31 indexes, 48 checks, 9 enums, 2 personal-data columns, 0 problems`
- Applied to an empty Postgres: yes (postgres:16): `schema-apply: docs/design/schema.sql applied to postgres:16, 19 tables`
