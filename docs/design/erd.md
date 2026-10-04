# Entity relationship diagram: Catalift

PostgreSQL. 19 tables, 23 relationships. Written beside docs/design/data-model.md and docs/design/schema.sql; the reason for every relationship is in data-model.md section 3.

## Diagram

```mermaid
erDiagram
    users ||--o{ sessions : "user_id"
    users ||--o{ uploads : "uploaded_by"
    uploads ||--o{ upload_row_errors : "upload_id"
    brands ||--o{ products : "brand_id"
    uploads ||--o{ products : "upload_id"
    products ||--o{ product_images : "product_id"
    products ||--o| product_attributes : "product_id"
    users |o--o{ product_attributes : "corrected_by"
    users ||--o{ generation_runs : "started_by"
    products ||--o{ listings : "product_id"
    listings ||--o{ rule_results : "listing_id"
    listings ||--o{ approvals : "listing_id"
    users ||--o{ approvals : "approved_by"
    listings ||--o{ regeneration_requests : "listing_id"
    users ||--o{ regeneration_requests : "requested_by"
    generation_runs |o--o{ jobs : "run_id"
    products ||--o{ jobs : "product_id"
    listings |o--o{ jobs : "listing_id"
    regeneration_requests |o--o{ jobs : "regeneration_request_id"
    jobs |o--o{ ai_calls : "job_id"
    products |o--o{ ai_calls : "product_id"
    users ||--o{ exports : "created_by"
    exports ||--o{ export_files : "export_id"
    users {
        bigint id PK
        text email UK "personal data"
        user_role role
        text password_hash "personal data"
        timestamptz created_at
        timestamptz updated_at
    }
    sessions {
        text token_hash PK
        bigint user_id FK
        text csrf_token_hash
        timestamptz expires_at
        timestamptz created_at
    }
    brands {
        bigint id PK
        text name UK
        text voice_note
        timestamptz created_at
        timestamptz updated_at
    }
    uploads {
        bigint id PK
        bigint uploaded_by FK
        text file_name
        integer rows_total
        integer rows_accepted
        integer rows_rejected
        timestamptz created_at
        timestamptz updated_at
    }
    upload_row_errors {
        bigint id PK
        bigint upload_id FK
        integer row_number
        text sku
        text reason
        timestamptz created_at
    }
    products {
        bigint id PK
        text sku UK
        bigint brand_id FK
        bigint upload_id FK
        text category
        bigint price_minor
        char3 currency
        timestamptz created_at
        timestamptz updated_at
    }
    product_images {
        bigint id PK
        bigint product_id FK
        smallint position
        text original_file_name
        text original_path
        text detection_path
        text content_type
        integer byte_size
        timestamptz created_at
    }
    product_attributes {
        bigint product_id PK, FK
        detection_status detection_status
        integer revision
        text colour
        text pattern
        text sleeve
        text neckline
        text fit
        text detection_error
        bigint corrected_by FK
        timestamptz created_at
        timestamptz updated_at
    }
    generation_runs {
        bigint id PK
        bigint started_by FK
        boolean neutral_voice_confirmed
        timestamptz created_at
        timestamptz updated_at
    }
    listings {
        bigint id PK
        bigint product_id FK
        text channel
        listing_status status
        text failure_reason
        integer version
        text title
        text bullet_1
        text bullet_2
        text bullet_3
        text bullet_4
        text bullet_5
        text description
        rule_status rule_status
        text rule_config_hash
        boolean first_pass_passed
        text_array first_pass_failed_rules
        timestamptz created_at
        timestamptz updated_at
    }
    rule_results {
        bigint id PK
        bigint listing_id FK
        integer listing_version
        text rule
        text field
        text message
        text config_hash
        timestamptz created_at
    }
    approvals {
        bigint id PK
        bigint listing_id FK
        integer listing_version
        bigint approved_by FK
        timestamptz created_at
    }
    regeneration_requests {
        bigint id PK
        bigint listing_id FK
        text field
        text instruction
        text base_value
        regeneration_status status
        bigint requested_by FK
        timestamptz created_at
        timestamptz updated_at
    }
    jobs {
        bigint id PK
        job_type type
        text dedupe_key
        job_status status
        bigint run_id FK
        bigint product_id FK
        bigint listing_id FK
        bigint regeneration_request_id FK
        smallint attempts
        timestamptz run_after
        timestamptz first_attempt_at
        integer attributes_revision
        uuid claim_token
        text claimed_by
        timestamptz lease_until
        text last_error
        timestamptz created_at
        timestamptz updated_at
        timestamptz finished_at
    }
    ai_calls {
        bigint id PK
        bigint job_id FK
        bigint product_id FK
        ai_call_purpose purpose
        text model
        text prompt_template_id
        text prompt
        ai_call_status status
        bigint reserved_micro_usd
        bigint cost_micro_usd
        integer input_tokens
        integer output_tokens
        text provider_generation_id
        text error
        timestamptz created_at
        timestamptz updated_at
    }
    budget {
        smallint id PK
        bigint limit_micro_usd
        timestamptz blocked_at
        timestamptz created_at
        timestamptz updated_at
    }
    exports {
        bigint id PK
        bigint created_by FK
        timestamptz created_at
    }
    export_files {
        bigint id PK
        bigint export_id FK
        text channel
        text file_path
        integer row_count
        timestamptz created_at
    }
    config_rechecks {
        bigint id PK
        text channel
        text config_hash
        integer listings_rechecked
        integer approvals_cleared
        timestamptz created_at
    }
```

## Relationships

- `users` to `sessions` through `sessions.user_id` (one-to-many, ON DELETE CASCADE): Deleting a user ends their sessions; a session without its user is meaningless.
- `users` to `uploads` through `uploads.uploaded_by` (one-to-many, ON DELETE RESTRICT): A user who uploaded cannot be deleted silently; uploads name who loaded the launch.
- `uploads` to `upload_row_errors` through `upload_row_errors.upload_id` (one-to-many, ON DELETE CASCADE): Row errors exist only to explain their upload and go with it.
- `brands` to `products` through `products.brand_id` (one-to-many, ON DELETE RESTRICT): A brand with products cannot be deleted; the voice note and listings depend on it.
- `uploads` to `products` through `products.upload_id` (one-to-many, ON DELETE RESTRICT): An upload with products cannot be deleted; images attach through it.
- `products` to `product_images` through `product_images.product_id` (one-to-many, ON DELETE RESTRICT): No story deletes products; an image never outlives its product unnoticed.
- `products` to `product_attributes` through `product_attributes.product_id` (one-to-zero-or-one, ON DELETE RESTRICT): One attribute row per product; no story deletes either.
- `users` to `product_attributes` through `product_attributes.corrected_by` (many-to-zero-or-one, ON DELETE RESTRICT): A reviewer who corrected attributes stays named; erase their email instead of deleting (open concern).
- `users` to `generation_runs` through `generation_runs.started_by` (one-to-many, ON DELETE RESTRICT): Runs name who started them.
- `products` to `listings` through `listings.product_id` (one-to-many, ON DELETE RESTRICT): A product with listings cannot be deleted; listings are the product's output.
- `listings` to `rule_results` through `rule_results.listing_id` (one-to-many, ON DELETE CASCADE): Rule results are derived from their listing and go with it.
- `listings` to `approvals` through `approvals.listing_id` (one-to-many, ON DELETE RESTRICT): Approvals are the audit trail of REQ-015 and must not vanish with a listing.
- `users` to `approvals` through `approvals.approved_by` (one-to-many, ON DELETE RESTRICT): An approval keeps naming its reviewer (ADR-0006).
- `listings` to `regeneration_requests` through `regeneration_requests.listing_id` (one-to-many, ON DELETE RESTRICT): A request keeps its listing; no story deletes listings.
- `users` to `regeneration_requests` through `regeneration_requests.requested_by` (one-to-many, ON DELETE RESTRICT): A request names the reviewer who asked.
- `generation_runs` to `jobs` through `jobs.run_id` (many-to-zero-or-one, ON DELETE RESTRICT): A run with jobs cannot be deleted; progress is read through it.
- `products` to `jobs` through `jobs.product_id` (one-to-many, ON DELETE RESTRICT): Jobs refer to live products; no story deletes products.
- `listings` to `jobs` through `jobs.listing_id` (many-to-zero-or-one, ON DELETE RESTRICT): Jobs refer to live listings; no story deletes listings.
- `regeneration_requests` to `jobs` through `jobs.regeneration_request_id` (many-to-zero-or-one, ON DELETE RESTRICT): A job serves its request; the request is kept.
- `jobs` to `ai_calls` through `ai_calls.job_id` (many-to-zero-or-one, ON DELETE SET NULL): The cost ledger outlives purged jobs; the link is cleared, the cost stays.
- `products` to `ai_calls` through `ai_calls.product_id` (many-to-zero-or-one, ON DELETE RESTRICT): A product's cost history must not vanish (AC-US-00-011-3).
- `users` to `exports` through `exports.created_by` (one-to-many, ON DELETE RESTRICT): An export names the reviewer who made it.
- `exports` to `export_files` through `export_files.export_id` (one-to-many, ON DELETE CASCADE): Files belong to their export and go with it.

`budget` and `config_rechecks` have no relationship: `budget` is a single row the AI gateway locks for the USD 8 check (REQ-021), and `config_rechecks` records a channel configuration change, which lives in files, not tables (D7).
