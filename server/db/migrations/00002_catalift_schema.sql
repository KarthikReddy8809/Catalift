-- +goose Up
-- The Catalift schema, from docs/design/schema.sql (data model v1, reviewed
-- 2026-10-04). Every table, column and index is explained in
-- docs/design/data-model.md; change it with a new migration, never here.
SET LOCAL lock_timeout = '2s';
SET LOCAL statement_timeout = '60s';

CREATE TYPE user_role AS ENUM ('seller', 'reviewer');
CREATE TYPE detection_status AS ENUM ('pending', 'done', 'failed', 'stopped_budget');
CREATE TYPE listing_status AS ENUM ('queued', 'generated', 'failed', 'stopped_budget');
CREATE TYPE rule_status AS ENUM ('unchecked', 'passing', 'failing');
CREATE TYPE regeneration_status AS ENUM ('queued', 'applied', 'superseded', 'failed', 'stopped_budget');
CREATE TYPE job_type AS ENUM ('detect_attributes', 'generate_listing', 'regenerate_field');
CREATE TYPE job_status AS ENUM ('queued', 'running', 'done', 'failed', 'stopped_budget', 'cancelled');
CREATE TYPE ai_call_purpose AS ENUM ('detect', 'generate', 'regenerate', 'eval_detect');
CREATE TYPE ai_call_status AS ENUM ('reserved', 'succeeded', 'failed', 'possibly_charged');

-- users: A seeded person who signs in as a seller or a reviewer.
-- Serves ADR-0006, US-00-009, US-00-010
CREATE TABLE users (
    id bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
    email text NOT NULL,
    role user_role NOT NULL,
    password_hash text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT users_pkey PRIMARY KEY (id),
    CONSTRAINT chk_users_email_shape CHECK (email ~ '^[^@[:space:]]+@[^@[:space:]]+$' AND length(email) <= 254)
);
COMMENT ON TABLE users IS 'A seeded person who signs in as a seller or a reviewer. Serves ADR-0006, US-00-009, US-00-010.';
COMMENT ON COLUMN users.id IS 'Surrogate key; referenced by sessions, uploads, runs, approvals and requests.';
COMMENT ON COLUMN users.email IS 'Sign-in address, unique ignoring case. [personal data]';
COMMENT ON COLUMN users.role IS 'seller or reviewer; checked by the API on every route.';
COMMENT ON COLUMN users.password_hash IS 'Password hash (argon2id or bcrypt, chosen in the LLD), never the password. [personal data]';
COMMENT ON COLUMN users.created_at IS 'When the row was written.';
COMMENT ON COLUMN users.updated_at IS 'Last change to this row, set by the repository on every update.';
-- Login looks a user up by email ignoring case; two accounts for one address are refused.
CREATE UNIQUE INDEX uq_users_email ON users (lower(email));

-- sessions: A signed-in browser session, carried by a secure HTTP-only cookie.
-- Serves ADR-0006
CREATE TABLE sessions (
    token_hash text NOT NULL,
    user_id bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    csrf_token_hash text NOT NULL,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT sessions_pkey PRIMARY KEY (token_hash),
    CONSTRAINT chk_sessions_expiry_after_creation CHECK (expires_at > created_at)
);
COMMENT ON TABLE sessions IS 'A signed-in browser session. Serves ADR-0006.';
COMMENT ON COLUMN sessions.token_hash IS 'SHA-256 of the session cookie token; the token itself is never stored.';
COMMENT ON COLUMN sessions.user_id IS 'The signed-in user.';
COMMENT ON COLUMN sessions.csrf_token_hash IS 'Hash of the CSRF token every state-changing request must send.';
COMMENT ON COLUMN sessions.expires_at IS 'When the session stops being accepted.';
COMMENT ON COLUMN sessions.created_at IS 'When the row was written.';
-- FK index: a user's sessions, and no scan when a user row is deleted.
CREATE INDEX idx_sessions_user_id ON sessions (user_id);
-- The daily sweep deletes sessions WHERE expires_at < now().
CREATE INDEX idx_sessions_expires_at ON sessions (expires_at);

-- brands: A brand named in the upload CSV, with its voice note.
-- Serves US-00-001, US-00-003
CREATE TABLE brands (
    id bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
    name text NOT NULL,
    voice_note text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT brands_pkey PRIMARY KEY (id),
    CONSTRAINT chk_brands_name_length CHECK (length(btrim(name)) BETWEEN 1 AND 100),
    CONSTRAINT chk_brands_voice_note_length CHECK (voice_note IS NULL OR length(voice_note) BETWEEN 1 AND 2000)
);
COMMENT ON TABLE brands IS 'A brand named in the upload CSV, with its voice note. Serves US-00-001, US-00-003.';
COMMENT ON COLUMN brands.id IS 'Surrogate key; referenced by products.';
COMMENT ON COLUMN brands.name IS 'Brand name as written in the CSV, unique ignoring case.';
COMMENT ON COLUMN brands.voice_note IS 'Short note on the brand''s voice; null until the seller writes one.';
COMMENT ON COLUMN brands.created_at IS 'When the row was written.';
COMMENT ON COLUMN brands.updated_at IS 'Last change to this row, set by the repository on every update.';
-- An upload finds the brand for a CSV row by name ignoring case and outer spaces.
CREATE UNIQUE INDEX uq_brands_name ON brands (lower(btrim(name)));

-- uploads: One CSV upload by a seller, with its row counts.
-- Serves US-00-001
CREATE TABLE uploads (
    id bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
    uploaded_by bigint NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    file_name text NOT NULL,
    rows_total integer NOT NULL,
    rows_accepted integer NOT NULL,
    rows_rejected integer NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uploads_pkey PRIMARY KEY (id),
    CONSTRAINT chk_uploads_counts CHECK (rows_accepted >= 0 AND rows_rejected >= 0 AND rows_accepted + rows_rejected = rows_total),
    CONSTRAINT chk_uploads_file_name_length CHECK (length(file_name) BETWEEN 1 AND 255)
);
COMMENT ON TABLE uploads IS 'One CSV upload by a seller. Serves US-00-001.';
COMMENT ON COLUMN uploads.id IS 'Surrogate key; the upload images are attached to.';
COMMENT ON COLUMN uploads.uploaded_by IS 'The seller who uploaded.';
COMMENT ON COLUMN uploads.file_name IS 'Name of the uploaded CSV file.';
COMMENT ON COLUMN uploads.rows_total IS 'Data rows in the file.';
COMMENT ON COLUMN uploads.rows_accepted IS 'Rows that became products.';
COMMENT ON COLUMN uploads.rows_rejected IS 'Rows refused, each listed in upload_row_errors.';
COMMENT ON COLUMN uploads.created_at IS 'When the row was written.';
COMMENT ON COLUMN uploads.updated_at IS 'Last change to this row, set by the repository on every update.';
-- FK index: no scan when a user row is checked on delete.
CREATE INDEX idx_uploads_uploaded_by ON uploads (uploaded_by);

-- upload_row_errors: A CSV row the upload refused, with its reason.
-- Serves US-00-001
CREATE TABLE upload_row_errors (
    id bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
    upload_id bigint NOT NULL REFERENCES uploads (id) ON DELETE CASCADE,
    row_number integer NOT NULL,
    sku text,
    reason text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT upload_row_errors_pkey PRIMARY KEY (id),
    CONSTRAINT chk_upload_row_errors_row_number CHECK (row_number >= 2),
    CONSTRAINT chk_upload_row_errors_reason_length CHECK (length(reason) BETWEEN 1 AND 500)
);
COMMENT ON TABLE upload_row_errors IS 'A CSV row the upload refused, with its reason. Serves US-00-001.';
COMMENT ON COLUMN upload_row_errors.id IS 'Surrogate key.';
COMMENT ON COLUMN upload_row_errors.upload_id IS 'The upload the row came from.';
COMMENT ON COLUMN upload_row_errors.row_number IS 'Line number in the CSV file, the header being line 1.';
COMMENT ON COLUMN upload_row_errors.sku IS 'SKU on the refused row; null when the row had none.';
COMMENT ON COLUMN upload_row_errors.reason IS 'Why the row was refused, as shown to the seller.';
COMMENT ON COLUMN upload_row_errors.created_at IS 'When the row was written.';
-- FK index and the upload summary: an upload's rejected rows.
CREATE INDEX idx_upload_row_errors_upload_id ON upload_row_errors (upload_id);

-- products: One SKU from the CSV: the thing listings describe.
-- Serves US-00-001, US-00-002, US-00-003, US-00-011
CREATE TABLE products (
    id bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
    sku text NOT NULL,
    brand_id bigint NOT NULL REFERENCES brands (id) ON DELETE RESTRICT,
    upload_id bigint NOT NULL REFERENCES uploads (id) ON DELETE RESTRICT,
    category text NOT NULL,
    price_minor bigint NOT NULL,
    currency char(3) NOT NULL DEFAULT 'INR',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT products_pkey PRIMARY KEY (id),
    CONSTRAINT chk_products_sku_shape CHECK (sku ~ '^[^[:space:]]{1,64}$'),
    CONSTRAINT chk_products_category_length CHECK (length(btrim(category)) BETWEEN 1 AND 100),
    CONSTRAINT chk_products_price_positive CHECK (price_minor > 0),
    CONSTRAINT chk_products_currency_iso CHECK (currency ~ '^[A-Z]{3}$')
);
COMMENT ON TABLE products IS 'One SKU from the CSV. Serves US-00-001, US-00-002, US-00-003, US-00-011.';
COMMENT ON COLUMN products.id IS 'Surrogate key; referenced by images, attributes, listings, jobs and AI calls.';
COMMENT ON COLUMN products.sku IS 'Seller''s product code, unique ignoring case across Catalift.';
COMMENT ON COLUMN products.brand_id IS 'The product''s brand.';
COMMENT ON COLUMN products.upload_id IS 'The upload that created the product.';
COMMENT ON COLUMN products.category IS 'Category from the CSV.';
COMMENT ON COLUMN products.price_minor IS 'Price in minor units of currency (paise).';
COMMENT ON COLUMN products.currency IS 'ISO 4217 code of price_minor.';
COMMENT ON COLUMN products.created_at IS 'When the row was written.';
COMMENT ON COLUMN products.updated_at IS 'Last change to this row, set by the repository on every update.';
-- D21: an upload rejects a SKU already present; D20 image matching looks SKUs up.
CREATE UNIQUE INDEX uq_products_sku ON products (lower(sku));
-- FK index; generation reads a brand's products.
CREATE INDEX idx_products_brand_id ON products (brand_id);
-- FK index; image upload lists an upload's SKUs.
CREATE INDEX idx_products_upload_id ON products (upload_id);

-- product_images: A photo attached to a product, with its 1024 px detection copy.
-- Serves US-00-001, US-00-002
CREATE TABLE product_images (
    id bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
    product_id bigint NOT NULL REFERENCES products (id) ON DELETE RESTRICT,
    position smallint NOT NULL,
    original_file_name text NOT NULL,
    original_path text NOT NULL,
    detection_path text NOT NULL,
    content_type text NOT NULL,
    byte_size integer NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT product_images_pkey PRIMARY KEY (id),
    CONSTRAINT chk_product_images_position CHECK (position >= 1),
    CONSTRAINT chk_product_images_content_type CHECK (content_type IN ('image/jpeg', 'image/png', 'image/webp')),
    CONSTRAINT chk_product_images_byte_size CHECK (byte_size BETWEEN 1 AND 10485760),
    CONSTRAINT chk_product_images_path_length CHECK (length(original_file_name) BETWEEN 1 AND 255 AND length(original_path) BETWEEN 1 AND 500 AND length(detection_path) BETWEEN 1 AND 500)
);
COMMENT ON TABLE product_images IS 'A photo attached to a product, with its detection copy. Serves US-00-001, US-00-002.';
COMMENT ON COLUMN product_images.id IS 'Surrogate key.';
COMMENT ON COLUMN product_images.product_id IS 'The product the image belongs to.';
COMMENT ON COLUMN product_images.position IS 'Order among the product''s images; 1 drives detection.';
COMMENT ON COLUMN product_images.original_file_name IS 'File name as uploaded.';
COMMENT ON COLUMN product_images.original_path IS 'Where the original is stored on the data disk, under a generated name.';
COMMENT ON COLUMN product_images.detection_path IS 'Where the copy scaled to at most 1024 px on its long side is stored.';
COMMENT ON COLUMN product_images.content_type IS 'Sniffed image type.';
COMMENT ON COLUMN product_images.byte_size IS 'Size of the original in bytes.';
COMMENT ON COLUMN product_images.created_at IS 'When the row was written.';
-- FK index, and detection reads the product's image at position 1; two images cannot both be first.
CREATE UNIQUE INDEX uq_product_images_product_position ON product_images (product_id, position);

-- product_attributes: The five detected attributes of a product, owned by listings (D5).
-- Serves US-00-002, US-00-007
CREATE TABLE product_attributes (
    product_id bigint NOT NULL REFERENCES products (id) ON DELETE RESTRICT,
    detection_status detection_status NOT NULL DEFAULT 'pending',
    revision integer NOT NULL DEFAULT 0,
    colour text,
    pattern text,
    sleeve text,
    neckline text,
    fit text,
    detection_error text,
    corrected_by bigint REFERENCES users (id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT product_attributes_pkey PRIMARY KEY (product_id),
    CONSTRAINT chk_product_attributes_done_complete CHECK (detection_status <> 'done' OR (colour IS NOT NULL AND pattern IS NOT NULL AND sleeve IS NOT NULL AND neckline IS NOT NULL AND fit IS NOT NULL)),
    CONSTRAINT chk_product_attributes_error_iff_failed CHECK ((detection_status = 'failed') = (detection_error IS NOT NULL)),
    CONSTRAINT chk_product_attributes_revision CHECK (revision >= 0),
    CONSTRAINT chk_product_attributes_value_length CHECK (coalesce(length(colour), 1) BETWEEN 1 AND 100 AND coalesce(length(pattern), 1) BETWEEN 1 AND 100 AND coalesce(length(sleeve), 1) BETWEEN 1 AND 100 AND coalesce(length(neckline), 1) BETWEEN 1 AND 100 AND coalesce(length(fit), 1) BETWEEN 1 AND 100)
);
COMMENT ON TABLE product_attributes IS 'The five detected attributes of a product, owned by listings. Serves US-00-002, US-00-007.';
COMMENT ON COLUMN product_attributes.product_id IS 'The product described; one row each.';
COMMENT ON COLUMN product_attributes.detection_status IS 'pending, done, failed or stopped_budget.';
COMMENT ON COLUMN product_attributes.revision IS 'Raised by every detection result and every reviewer correction.';
COMMENT ON COLUMN product_attributes.colour IS 'Detected or corrected colour; ''unknown'' when unclear; null until done.';
COMMENT ON COLUMN product_attributes.pattern IS 'Detected or corrected pattern; null until done.';
COMMENT ON COLUMN product_attributes.sleeve IS 'Detected or corrected sleeve; null until done.';
COMMENT ON COLUMN product_attributes.neckline IS 'Detected or corrected neckline; null until done.';
COMMENT ON COLUMN product_attributes.fit IS 'Detected or corrected fit; null until done.';
COMMENT ON COLUMN product_attributes.detection_error IS 'Why detection failed; null unless failed.';
COMMENT ON COLUMN product_attributes.corrected_by IS 'Reviewer who last corrected an attribute; null while only detected.';
COMMENT ON COLUMN product_attributes.created_at IS 'When the row was written.';
COMMENT ON COLUMN product_attributes.updated_at IS 'Last change to this row, set by the repository on every update.';
-- FK index: no scan when a user row is checked on delete.
CREATE INDEX idx_product_attributes_corrected_by ON product_attributes (corrected_by);

-- generation_runs: One start of generation by a seller.
-- Serves US-00-003, US-00-012
CREATE TABLE generation_runs (
    id bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
    started_by bigint NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    neutral_voice_confirmed boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT generation_runs_pkey PRIMARY KEY (id)
);
COMMENT ON TABLE generation_runs IS 'One start of generation by a seller. Serves US-00-003, US-00-012.';
COMMENT ON COLUMN generation_runs.id IS 'Surrogate key; jobs and the progress view group by it.';
COMMENT ON COLUMN generation_runs.started_by IS 'Who started the run.';
COMMENT ON COLUMN generation_runs.neutral_voice_confirmed IS 'True when the seller confirmed a neutral voice for brands with no voice note.';
COMMENT ON COLUMN generation_runs.created_at IS 'When the row was written.';
COMMENT ON COLUMN generation_runs.updated_at IS 'Last change to this row, set by the repository on every update.';
-- FK index: no scan when a user row is checked on delete.
CREATE INDEX idx_generation_runs_started_by ON generation_runs (started_by);

-- listings: One product on one channel: its text, version and rule status.
-- Serves US-00-003, US-00-004, US-00-006, US-00-007, US-00-008, US-00-009, US-00-010
CREATE TABLE listings (
    id bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
    product_id bigint NOT NULL REFERENCES products (id) ON DELETE RESTRICT,
    channel text NOT NULL,
    status listing_status NOT NULL DEFAULT 'queued',
    failure_reason text,
    version integer NOT NULL DEFAULT 0,
    title text,
    bullet_1 text,
    bullet_2 text,
    bullet_3 text,
    bullet_4 text,
    bullet_5 text,
    description text,
    rule_status rule_status NOT NULL DEFAULT 'unchecked',
    rule_config_hash text,
    first_pass_passed boolean,
    first_pass_failed_rules text[],
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT listings_pkey PRIMARY KEY (id),
    CONSTRAINT chk_listings_channel_shape CHECK (channel ~ '^[a-z][a-z0-9_]{1,39}$'),
    CONSTRAINT chk_listings_version CHECK (version >= 0),
    CONSTRAINT chk_listings_generated_complete CHECK (status <> 'generated' OR (version >= 1 AND title IS NOT NULL AND bullet_1 IS NOT NULL AND bullet_2 IS NOT NULL AND bullet_3 IS NOT NULL AND bullet_4 IS NOT NULL AND bullet_5 IS NOT NULL AND description IS NOT NULL)),
    CONSTRAINT chk_listings_failure_reason CHECK ((status = 'failed') = (failure_reason IS NOT NULL)),
    CONSTRAINT chk_listings_text_length CHECK (coalesce(length(title), 1) BETWEEN 1 AND 500 AND coalesce(length(bullet_1), 1) BETWEEN 1 AND 1000 AND coalesce(length(bullet_2), 1) BETWEEN 1 AND 1000 AND coalesce(length(bullet_3), 1) BETWEEN 1 AND 1000 AND coalesce(length(bullet_4), 1) BETWEEN 1 AND 1000 AND coalesce(length(bullet_5), 1) BETWEEN 1 AND 1000 AND coalesce(length(description), 1) BETWEEN 1 AND 10000),
    CONSTRAINT chk_listings_first_pass_pair CHECK ((first_pass_passed IS NULL AND first_pass_failed_rules IS NULL) OR (first_pass_passed = (cardinality(first_pass_failed_rules) = 0))),
    CONSTRAINT chk_listings_generated_checked CHECK (status <> 'generated' OR rule_status <> 'unchecked')
);
COMMENT ON TABLE listings IS 'One product on one channel: text, version and rule status. Serves US-00-003, US-00-004, US-00-006 to US-00-010.';
COMMENT ON COLUMN listings.id IS 'Surrogate key; the grid row and the URL of a listing.';
COMMENT ON COLUMN listings.product_id IS 'The product described.';
COMMENT ON COLUMN listings.channel IS 'Channel id from configuration, such as amazon_style.';
COMMENT ON COLUMN listings.status IS 'queued, generated, failed or stopped_budget.';
COMMENT ON COLUMN listings.failure_reason IS 'Why generation failed; null unless failed.';
COMMENT ON COLUMN listings.version IS '0 before generation; raised by every change to the text or the product''s attributes.';
COMMENT ON COLUMN listings.title IS 'Generated or edited title; null until generated.';
COMMENT ON COLUMN listings.bullet_1 IS 'Bullet point 1; null until generated.';
COMMENT ON COLUMN listings.bullet_2 IS 'Bullet point 2; null until generated.';
COMMENT ON COLUMN listings.bullet_3 IS 'Bullet point 3; null until generated.';
COMMENT ON COLUMN listings.bullet_4 IS 'Bullet point 4; null until generated.';
COMMENT ON COLUMN listings.bullet_5 IS 'Bullet point 5; null until generated.';
COMMENT ON COLUMN listings.description IS 'Generated or edited description; null until generated.';
COMMENT ON COLUMN listings.rule_status IS 'unchecked, passing or failing for the current version.';
COMMENT ON COLUMN listings.rule_config_hash IS 'Hash of the channel config the current status was checked against.';
COMMENT ON COLUMN listings.first_pass_passed IS 'Whether the first generated version passed the rules; set once.';
COMMENT ON COLUMN listings.first_pass_failed_rules IS 'Rule names the first version failed; set once with first_pass_passed.';
COMMENT ON COLUMN listings.created_at IS 'When the row was written.';
COMMENT ON COLUMN listings.updated_at IS 'Last change to this row, set by the repository on every update.';
-- FK index; one listing per product and channel; re-runs (D6) find the pair to re-queue.
CREATE UNIQUE INDEX uq_listings_product_channel ON listings (product_id, channel);

-- rule_results: One rule failure of one listing version.
-- Serves US-00-004, US-00-007
CREATE TABLE rule_results (
    id bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
    listing_id bigint NOT NULL REFERENCES listings (id) ON DELETE CASCADE,
    listing_version integer NOT NULL,
    rule text NOT NULL,
    field text NOT NULL,
    message text NOT NULL,
    config_hash text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT rule_results_pkey PRIMARY KEY (id),
    CONSTRAINT chk_rule_results_text_length CHECK (length(rule) BETWEEN 1 AND 100 AND length(field) BETWEEN 1 AND 100 AND length(message) BETWEEN 1 AND 1000),
    CONSTRAINT chk_rule_results_version CHECK (listing_version >= 1)
);
COMMENT ON TABLE rule_results IS 'One rule failure of one listing version. Serves US-00-004, US-00-007.';
COMMENT ON COLUMN rule_results.id IS 'Surrogate key.';
COMMENT ON COLUMN rule_results.listing_id IS 'The listing that failed.';
COMMENT ON COLUMN rule_results.listing_version IS 'Version the rule was checked against.';
COMMENT ON COLUMN rule_results.rule IS 'Rule name, such as title_max_length.';
COMMENT ON COLUMN rule_results.field IS 'Field the failure is in, such as title or bullet_3.';
COMMENT ON COLUMN rule_results.message IS 'Detail shown to the reviewer: limit and length, the word, or the missing attribute.';
COMMENT ON COLUMN rule_results.config_hash IS 'Hash of the channel config used.';
COMMENT ON COLUMN rule_results.created_at IS 'When the row was written.';
-- FK index; the grid and the approval check read a listing's failures for its current version.
CREATE INDEX idx_rule_results_listing_version ON rule_results (listing_id, listing_version);

-- approvals: A reviewer's approval of one listing version.
-- Serves US-00-009, US-00-010
CREATE TABLE approvals (
    id bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
    listing_id bigint NOT NULL REFERENCES listings (id) ON DELETE RESTRICT,
    listing_version integer NOT NULL,
    approved_by bigint NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT approvals_pkey PRIMARY KEY (id),
    CONSTRAINT chk_approvals_version CHECK (listing_version >= 1)
);
COMMENT ON TABLE approvals IS 'A reviewer''s approval of one listing version. Serves US-00-009, US-00-010.';
COMMENT ON COLUMN approvals.id IS 'Surrogate key.';
COMMENT ON COLUMN approvals.listing_id IS 'The listing approved.';
COMMENT ON COLUMN approvals.listing_version IS 'Version approved; the listing counts as approved only while its version still equals this.';
COMMENT ON COLUMN approvals.approved_by IS 'The reviewer who approved.';
COMMENT ON COLUMN approvals.created_at IS 'When the row was written.';
-- FK index; export and the grid join approvals on (listing_id, version); a version is approved once.
CREATE UNIQUE INDEX uq_approvals_listing_version ON approvals (listing_id, listing_version);
-- FK index: no scan when a user row is checked on delete.
CREATE INDEX idx_approvals_approved_by ON approvals (approved_by);

-- regeneration_requests: A reviewer's request to rewrite one field with an instruction.
-- Serves US-00-008
CREATE TABLE regeneration_requests (
    id bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
    listing_id bigint NOT NULL REFERENCES listings (id) ON DELETE RESTRICT,
    field text NOT NULL,
    instruction text NOT NULL,
    base_value text NOT NULL,
    status regeneration_status NOT NULL DEFAULT 'queued',
    requested_by bigint NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT regeneration_requests_pkey PRIMARY KEY (id),
    CONSTRAINT chk_regeneration_requests_field CHECK (field IN ('title', 'bullet_1', 'bullet_2', 'bullet_3', 'bullet_4', 'bullet_5', 'description')),
    CONSTRAINT chk_regeneration_requests_instruction_length CHECK (length(btrim(instruction)) BETWEEN 1 AND 1000)
);
COMMENT ON TABLE regeneration_requests IS 'A reviewer''s request to rewrite one field. Serves US-00-008.';
COMMENT ON COLUMN regeneration_requests.id IS 'Surrogate key; the request id in the regenerate job key.';
COMMENT ON COLUMN regeneration_requests.listing_id IS 'The listing to change.';
COMMENT ON COLUMN regeneration_requests.field IS 'title, bullet_1 to bullet_5, or description.';
COMMENT ON COLUMN regeneration_requests.instruction IS 'Reviewer''s free-text instruction.';
COMMENT ON COLUMN regeneration_requests.base_value IS 'The field''s value when the request was made.';
COMMENT ON COLUMN regeneration_requests.status IS 'queued, applied, superseded, failed or stopped_budget.';
COMMENT ON COLUMN regeneration_requests.requested_by IS 'The reviewer who asked.';
COMMENT ON COLUMN regeneration_requests.created_at IS 'When the row was written.';
COMMENT ON COLUMN regeneration_requests.updated_at IS 'Last change to this row, set by the repository on every update.';
-- FK index; the grid shows a listing's open requests.
CREATE INDEX idx_regeneration_requests_listing_id ON regeneration_requests (listing_id);
-- FK index: no scan when a user row is checked on delete.
CREATE INDEX idx_regeneration_requests_requested_by ON regeneration_requests (requested_by);

-- jobs: One unit of background AI work, claimed by the worker (ADR-0005).
-- Serves US-00-002, US-00-003, US-00-008, US-00-012
CREATE TABLE jobs (
    id bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
    type job_type NOT NULL,
    dedupe_key text NOT NULL,
    status job_status NOT NULL DEFAULT 'queued',
    run_id bigint REFERENCES generation_runs (id) ON DELETE RESTRICT,
    product_id bigint NOT NULL REFERENCES products (id) ON DELETE RESTRICT,
    listing_id bigint REFERENCES listings (id) ON DELETE RESTRICT,
    regeneration_request_id bigint REFERENCES regeneration_requests (id) ON DELETE RESTRICT,
    attempts smallint NOT NULL DEFAULT 0,
    run_after timestamptz NOT NULL DEFAULT now(),
    first_attempt_at timestamptz,
    attributes_revision integer,
    claim_token uuid,
    claimed_by text,
    lease_until timestamptz,
    last_error text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    finished_at timestamptz,
    CONSTRAINT jobs_pkey PRIMARY KEY (id),
    CONSTRAINT chk_jobs_attempts CHECK (attempts BETWEEN 0 AND 3),
    CONSTRAINT chk_jobs_targets CHECK ((type = 'detect_attributes' AND listing_id IS NULL AND regeneration_request_id IS NULL AND attributes_revision IS NULL) OR (type = 'generate_listing' AND listing_id IS NOT NULL AND regeneration_request_id IS NULL AND attributes_revision IS NOT NULL) OR (type = 'regenerate_field' AND listing_id IS NOT NULL AND regeneration_request_id IS NOT NULL AND run_id IS NULL AND attributes_revision IS NOT NULL)),
    CONSTRAINT chk_jobs_claim CHECK ((status = 'running') = (claimed_by IS NOT NULL AND lease_until IS NOT NULL AND claim_token IS NOT NULL)),
    CONSTRAINT chk_jobs_finished CHECK ((status IN ('done', 'failed', 'stopped_budget', 'cancelled')) = (finished_at IS NOT NULL)),
    CONSTRAINT chk_jobs_text_length CHECK (length(dedupe_key) BETWEEN 1 AND 200 AND coalesce(length(last_error), 1) BETWEEN 1 AND 2000 AND coalesce(length(claimed_by), 1) BETWEEN 1 AND 100)
);
COMMENT ON TABLE jobs IS 'One unit of background AI work claimed by the worker. Serves US-00-002, US-00-003, US-00-008, US-00-012.';
COMMENT ON COLUMN jobs.id IS 'Surrogate key; logs carry it.';
COMMENT ON COLUMN jobs.type IS 'detect_attributes, generate_listing or regenerate_field.';
COMMENT ON COLUMN jobs.dedupe_key IS 'Product, step and channel, or listing, field and request id for regenerations.';
COMMENT ON COLUMN jobs.status IS 'queued, running, done, failed, stopped_budget or cancelled.';
COMMENT ON COLUMN jobs.run_id IS 'The run the job belongs to; null for regenerations.';
COMMENT ON COLUMN jobs.product_id IS 'The product the work is for.';
COMMENT ON COLUMN jobs.listing_id IS 'The listing written; null for detection.';
COMMENT ON COLUMN jobs.regeneration_request_id IS 'The request served; null unless a regeneration.';
COMMENT ON COLUMN jobs.attempts IS 'Attempts used, 0 to 3; a 429 does not use one.';
COMMENT ON COLUMN jobs.run_after IS 'Earliest time the job may be claimed.';
COMMENT ON COLUMN jobs.first_attempt_at IS 'When the first attempt started; null until claimed.';
COMMENT ON COLUMN jobs.attributes_revision IS 'Attribute revision read when the prompt was built; null for detection.';
COMMENT ON COLUMN jobs.claim_token IS 'Random token set on every claim; null unless running.';
COMMENT ON COLUMN jobs.claimed_by IS 'Worker process holding the claim; null unless running.';
COMMENT ON COLUMN jobs.lease_until IS 'When the claim expires; null unless running.';
COMMENT ON COLUMN jobs.last_error IS 'Most recent error; null when none.';
COMMENT ON COLUMN jobs.created_at IS 'When the row was written.';
COMMENT ON COLUMN jobs.updated_at IS 'Last change to this row, set by the repository on every update.';
COMMENT ON COLUMN jobs.finished_at IS 'When the job reached done, failed, stopped_budget or cancelled.';
-- The claim: SELECT ... WHERE status = 'queued' AND run_after <= now() ORDER BY run_after, id FOR UPDATE SKIP LOCKED.
CREATE INDEX idx_jobs_claimable ON jobs (run_after, id) WHERE status = 'queued';
-- The stale-claim sweep: WHERE status = 'running' AND lease_until < now().
CREATE INDEX idx_jobs_running_lease ON jobs (lease_until) WHERE status = 'running';
-- ADR-0005 check 6: a second enqueue of live work is refused, while finished work can be queued again.
CREATE UNIQUE INDEX uq_jobs_dedupe_key_active ON jobs (dedupe_key) WHERE status IN ('queued', 'running');
-- FK index; run progress and 'resume failed and stopped' (D6).
CREATE INDEX idx_jobs_run_id ON jobs (run_id);
-- FK index; per-product progress.
CREATE INDEX idx_jobs_product_id ON jobs (product_id);
-- FK index: no scan when a listing row is checked.
CREATE INDEX idx_jobs_listing_id ON jobs (listing_id);
-- FK index: a request's job.
CREATE INDEX idx_jobs_regeneration_request_id ON jobs (regeneration_request_id);

-- ai_calls: One call through the AI gateway: the cost ledger.
-- Serves US-00-011, US-00-012, US-00-003, US-00-008
CREATE TABLE ai_calls (
    id bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
    job_id bigint REFERENCES jobs (id) ON DELETE SET NULL,
    product_id bigint REFERENCES products (id) ON DELETE RESTRICT,
    purpose ai_call_purpose NOT NULL,
    model text NOT NULL,
    prompt_template_id text NOT NULL,
    prompt text NOT NULL,
    status ai_call_status NOT NULL DEFAULT 'reserved',
    reserved_micro_usd bigint NOT NULL,
    cost_micro_usd bigint,
    input_tokens integer,
    output_tokens integer,
    provider_generation_id text,
    error text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ai_calls_pkey PRIMARY KEY (id),
    CONSTRAINT chk_ai_calls_amounts CHECK (reserved_micro_usd > 0 AND coalesce(cost_micro_usd, 0) >= 0 AND coalesce(input_tokens, 0) >= 0 AND coalesce(output_tokens, 0) >= 0),
    CONSTRAINT chk_ai_calls_succeeded_complete CHECK (status <> 'succeeded' OR (cost_micro_usd IS NOT NULL AND input_tokens IS NOT NULL AND output_tokens IS NOT NULL)),
    CONSTRAINT chk_ai_calls_product_unless_eval CHECK ((purpose = 'eval_detect') = (product_id IS NULL)),
    CONSTRAINT chk_ai_calls_text_length CHECK (length(model) BETWEEN 1 AND 100 AND length(prompt_template_id) BETWEEN 1 AND 100 AND length(prompt) BETWEEN 1 AND 200000)
);
COMMENT ON TABLE ai_calls IS 'One call through the AI gateway; the cost ledger. Serves US-00-011, US-00-012.';
COMMENT ON COLUMN ai_calls.id IS 'Surrogate key.';
COMMENT ON COLUMN ai_calls.job_id IS 'The job that made the call; null for evals or after the job is purged.';
COMMENT ON COLUMN ai_calls.product_id IS 'The product the call was for; null only for eval calls.';
COMMENT ON COLUMN ai_calls.purpose IS 'detect, generate, regenerate or eval_detect.';
COMMENT ON COLUMN ai_calls.model IS 'Model id, such as claude-haiku-4-5.';
COMMENT ON COLUMN ai_calls.prompt_template_id IS 'Id of the prompt template rendered.';
COMMENT ON COLUMN ai_calls.prompt IS 'The rendered prompt sent.';
COMMENT ON COLUMN ai_calls.status IS 'reserved, succeeded, failed or possibly_charged.';
COMMENT ON COLUMN ai_calls.reserved_micro_usd IS 'Estimated cost reserved before the call, in millionths of a US dollar.';
COMMENT ON COLUMN ai_calls.cost_micro_usd IS 'Actual cost reported, in millionths of a US dollar; null until known.';
COMMENT ON COLUMN ai_calls.input_tokens IS 'Input tokens reported; null until known.';
COMMENT ON COLUMN ai_calls.output_tokens IS 'Output tokens reported; null until known.';
COMMENT ON COLUMN ai_calls.provider_generation_id IS 'OpenRouter''s generation id; null when not returned.';
COMMENT ON COLUMN ai_calls.error IS 'Error returned; null on success.';
COMMENT ON COLUMN ai_calls.created_at IS 'When the row was written.';
COMMENT ON COLUMN ai_calls.updated_at IS 'Last change to this row, set by the repository on every update.';
-- FK index; SET NULL on job purge does not scan the ledger.
CREATE INDEX idx_ai_calls_job_id ON ai_calls (job_id);
-- AC-US-00-011-3 and -4: cost per product.
CREATE INDEX idx_ai_calls_product_id ON ai_calls (product_id);
-- The sweep that turns a reservation left after the lease into possibly_charged.
CREATE INDEX idx_ai_calls_reserved ON ai_calls (created_at) WHERE status = 'reserved';

-- budget: The single row holding the AI spend limit and the blocked state.
-- Serves US-00-012
CREATE TABLE budget (
    id smallint NOT NULL DEFAULT 1,
    limit_micro_usd bigint NOT NULL DEFAULT 8000000,
    blocked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT budget_pkey PRIMARY KEY (id),
    CONSTRAINT chk_budget_single_row CHECK (id = 1),
    CONSTRAINT chk_budget_limit_positive CHECK (limit_micro_usd > 0)
);
COMMENT ON TABLE budget IS 'The single row holding the AI spend limit and blocked state. Serves US-00-012.';
COMMENT ON COLUMN budget.id IS 'Always 1; the row the gateway locks.';
COMMENT ON COLUMN budget.limit_micro_usd IS 'Spend after which calls are refused, in millionths of a US dollar.';
COMMENT ON COLUMN budget.blocked_at IS 'When the gateway first refused a call; null while not blocked.';
COMMENT ON COLUMN budget.created_at IS 'When the row was written.';
COMMENT ON COLUMN budget.updated_at IS 'Last change to this row, set by the repository on every update.';

-- exports: One export run by a reviewer.
-- Serves US-00-010
CREATE TABLE exports (
    id bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
    created_by bigint NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT exports_pkey PRIMARY KEY (id)
);
COMMENT ON TABLE exports IS 'One export run by a reviewer. Serves US-00-010.';
COMMENT ON COLUMN exports.id IS 'Surrogate key; the download URL names it.';
COMMENT ON COLUMN exports.created_by IS 'The reviewer who exported.';
COMMENT ON COLUMN exports.created_at IS 'When the row was written.';
-- FK index: no scan when a user row is checked on delete.
CREATE INDEX idx_exports_created_by ON exports (created_by);

-- export_files: One channel's CSV within an export.
-- Serves US-00-010
CREATE TABLE export_files (
    id bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
    export_id bigint NOT NULL REFERENCES exports (id) ON DELETE CASCADE,
    channel text NOT NULL,
    file_path text NOT NULL,
    row_count integer NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT export_files_pkey PRIMARY KEY (id),
    CONSTRAINT chk_export_files_row_count CHECK (row_count > 0),
    CONSTRAINT chk_export_files_channel_shape CHECK (channel ~ '^[a-z][a-z0-9_]{1,39}$'),
    CONSTRAINT chk_export_files_path_length CHECK (length(file_path) BETWEEN 1 AND 500)
);
COMMENT ON TABLE export_files IS 'One channel''s CSV within an export. Serves US-00-010.';
COMMENT ON COLUMN export_files.id IS 'Surrogate key.';
COMMENT ON COLUMN export_files.export_id IS 'The export the file belongs to.';
COMMENT ON COLUMN export_files.channel IS 'Channel id from configuration.';
COMMENT ON COLUMN export_files.file_path IS 'Where the CSV is stored on the data disk.';
COMMENT ON COLUMN export_files.row_count IS 'Approved listings in the file.';
COMMENT ON COLUMN export_files.created_at IS 'When the row was written.';
-- FK index; one file per channel per export.
CREATE UNIQUE INDEX uq_export_files_export_channel ON export_files (export_id, channel);

-- config_rechecks: One start-up re-check of listings after a channel's rules changed (D7).
-- Serves US-00-004, US-00-005
CREATE TABLE config_rechecks (
    id bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
    channel text NOT NULL,
    config_hash text NOT NULL,
    listings_rechecked integer NOT NULL,
    approvals_cleared integer NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT config_rechecks_pkey PRIMARY KEY (id),
    CONSTRAINT chk_config_rechecks_counts CHECK (listings_rechecked >= 0 AND approvals_cleared BETWEEN 0 AND listings_rechecked),
    CONSTRAINT chk_config_rechecks_channel_shape CHECK (channel ~ '^[a-z][a-z0-9_]{1,39}$' AND length(config_hash) BETWEEN 1 AND 128)
);
COMMENT ON TABLE config_rechecks IS 'One start-up re-check after a channel rule change. Serves US-00-004, US-00-005.';
COMMENT ON COLUMN config_rechecks.id IS 'Surrogate key.';
COMMENT ON COLUMN config_rechecks.channel IS 'Channel whose configuration changed.';
COMMENT ON COLUMN config_rechecks.config_hash IS 'Hash of the configuration now loaded.';
COMMENT ON COLUMN config_rechecks.listings_rechecked IS 'Listings validated again.';
COMMENT ON COLUMN config_rechecks.approvals_cleared IS 'Approved listings that now fail and lost their approval.';
COMMENT ON COLUMN config_rechecks.created_at IS 'When the row was written.';
-- The grid shows the latest re-check per channel.
CREATE INDEX idx_config_rechecks_channel_created ON config_rechecks (channel, created_at DESC);

-- The single budget row the gateway locks (REQ-021, D13).
INSERT INTO budget (id) VALUES (1);

-- +goose Down
DROP TABLE IF EXISTS config_rechecks;
DROP TABLE IF EXISTS export_files;
DROP TABLE IF EXISTS exports;
DROP TABLE IF EXISTS budget;
DROP TABLE IF EXISTS ai_calls;
DROP TABLE IF EXISTS jobs;
DROP TABLE IF EXISTS regeneration_requests;
DROP TABLE IF EXISTS approvals;
DROP TABLE IF EXISTS rule_results;
DROP TABLE IF EXISTS listings;
DROP TABLE IF EXISTS generation_runs;
DROP TABLE IF EXISTS product_attributes;
DROP TABLE IF EXISTS product_images;
DROP TABLE IF EXISTS products;
DROP TABLE IF EXISTS upload_row_errors;
DROP TABLE IF EXISTS uploads;
DROP TABLE IF EXISTS brands;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS users;
DROP TYPE IF EXISTS ai_call_status;
DROP TYPE IF EXISTS ai_call_purpose;
DROP TYPE IF EXISTS job_status;
DROP TYPE IF EXISTS job_type;
DROP TYPE IF EXISTS regeneration_status;
DROP TYPE IF EXISTS rule_status;
DROP TYPE IF EXISTS listing_status;
DROP TYPE IF EXISTS detection_status;
DROP TYPE IF EXISTS user_role;
