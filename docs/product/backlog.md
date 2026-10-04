# Backlog: Catalift

PRD: docs/product/PRD.md   Questions: docs/product/questions.md   Built: 2026-10-01

## Story index

| Story | Epic | Title | Persona | Priority | Points | Covers | Depends on |
| --- | --- | --- | --- | --- | --- | --- | --- |
| US-00-001 | EP-01 | Upload a product catalogue with images | Seller | Must | TBD | REQ-001, REQ-002 | none |
| US-00-002 | EP-01 | Detect product attributes from photos | Seller | Must | TBD | REQ-003 | US-00-001, US-00-011 |
| US-00-003 | EP-01 | Generate listings for each channel in the brand's voice | Seller | Must | TBD | REQ-004, REQ-005, REQ-006, REQ-007 | US-00-002, US-00-005, US-00-011 |
| US-00-005 | EP-02 | Set up channels and their rules as configuration | Catalogue reviewer | Must | TBD | REQ-017, REQ-018 | none |
| US-00-004 | EP-02 | Validate every listing against its channel's rules | Catalogue reviewer | Must | TBD | REQ-008, REQ-009, REQ-010 | US-00-003, US-00-005 |
| US-00-006 | EP-02 | Add a Flipkart-style channel through configuration only | Seller | Could | TBD | REQ-019 | US-00-005, US-00-004 |
| US-00-007 | EP-03 | Edit listings in a review grid | Catalogue reviewer | Must | TBD | REQ-011 | US-00-004 |
| US-00-008 | EP-03 | Regenerate one field with an instruction | Catalogue reviewer | Must | TBD | REQ-013 | US-00-007 |
| US-00-009 | EP-03 | Approve listings in bulk | Catalogue reviewer | Must | TBD | REQ-012 | US-00-004, US-00-007 |
| US-00-010 | EP-04 | Export approved listings as one CSV per channel | Catalogue reviewer | Must | TBD | REQ-014, REQ-015 | US-00-009 |
| US-00-011 | EP-05 | See the AI cost of each product | Seller | Must | TBD | REQ-016, REQ-020 | none |
| US-00-012 | EP-05 | Stop AI calls once the budget limit is passed | Seller | Must | TBD | REQ-021 | US-00-011 |

## Hours by discipline

tasks: not written (stories only)

## EP-01 Turn a product upload into draft listings

Goal: a seller uploads a CSV and photos and gets a draft title, 5 bullets and a description for every product on every enabled channel.
Covers: REQ-001, REQ-002, REQ-003, REQ-004, REQ-005, REQ-006, REQ-007

### US-00-001 Upload a product catalogue with images

Epic: EP-01   Priority: Must   Points: TBD (estimate)
Persona: Seller, group 00   Ticket: unassigned
Covers: REQ-001, REQ-002   Judgement: merged from REQ-001, REQ-002

**Narrative.** As a seller, I want to upload my product CSV and photos in one go, so that every SKU of a launch is in the system without typing.

**Why it matters.** B1: the month-long rewrite starts with getting 300 SKUs in; nothing downstream runs until products and their photos are loaded and matched.

**From the PRD.**
- REQ-001: "The system accepts a bulk CSV upload with one row per product carrying SKU, category, brand and price."
- REQ-002: "The system accepts product image uploads and associates each image with a product from the CSV."

**Preconditions.**
- The seller has a CSV with SKU, category, brand and price columns, and image files.

**Acceptance criteria.**

- AC-US-00-001-1. Given a CSV with SKU, category, brand and price on every row, when the seller uploads it, then one product per row is created and the upload summary shows the count.
  Covers: REQ-001
- AC-US-00-001-2. Given a CSV row with a missing required column value or a SKU repeated in the file, when the seller uploads it, then that row is rejected with its row number and reason and the valid rows are still created.
  Covers: REQ-001
- AC-US-00-001-3. Given images whose file names start with an uploaded SKU, when the seller uploads them, then each image is attached to that product and the product shows its image count.
  Covers: REQ-002
- AC-US-00-001-4. Given an image whose file name matches no SKU in the upload, when the seller uploads it, then it is listed as unmatched and attached to no product.
  Covers: REQ-002
- AC-US-00-001-5. Given a product with no matched image after upload, when the seller views the upload summary, then the product is flagged "missing image" and is not sent for detection.
  Covers: REQ-002

**Not in this story.**
- Attribute detection from the images (US-00-002).
- Editing or cropping images (PRD non-goal: image editing).
- Categories other than apparel (Q-006).

**Depends on.**
- none

**Assumptions.**
- Images are matched by SKU file-name prefix; several images per SKU are allowed and the first drives detection (Q-005).
- Invalid rows are rejected one by one, not the whole file (Q-020).
- A product without an image is not generated (Q-019).

**Tasks.** not written in this run

### US-00-002 Detect product attributes from photos

Epic: EP-01   Priority: Must   Points: TBD (estimate)
Persona: Seller, group 00   Ticket: unassigned
Covers: REQ-003   Judgement: story

**Narrative.** As a seller, I want colour, pattern, sleeve, neckline and fit read from each product photo, so that I do not type attributes for 300 SKUs.

**Why it matters.** B3: detection accuracy is one of the two success measures; B1: attributes typed by hand are a large part of today's month.

**From the PRD.**
- REQ-003: "The system detects colour, pattern, sleeve, neckline and fit from each product photo using AI."

**Preconditions.**
- The product has at least one matched image (US-00-001).

**Acceptance criteria.**

- AC-US-00-002-1. Given an uploaded product with an image, when detection runs, then the product stores a value for each of colour, pattern, sleeve, neckline and fit.
  Covers: REQ-003
- AC-US-00-002-2. Given detection runs on a batch of products, when it completes, then each product shows a detection status of done or failed, and a failed product shows the reason.
  Covers: REQ-003
- AC-US-00-002-3. Given an attribute the model cannot determine from the image, when detection returns, then that attribute is stored as "unknown" rather than a guessed value.
  Covers: REQ-003
- AC-US-00-002-4. Given detection calls the AI model, when the call is made, then it goes through the gateway and its tokens and cost are recorded against the product.
  Covers: REQ-003

**Not in this story.**
- Correcting a detected attribute by hand (US-00-007).
- Measuring accuracy on the 30 labelled products (Q-024).
- Non-apparel attribute sets (Q-006).

**Depends on.**
- US-00-001: products and their images.
- US-00-011: the gateway that records cost per call.

**Assumptions.**
- Products are apparel only (Q-006).
- Detected attributes are editable later (Q-016).

**Tasks.** not written in this run

### US-00-003 Generate listings for each channel in the brand's voice

Epic: EP-01   Priority: Must   Points: TBD (estimate)
Persona: Seller, group 00   Ticket: unassigned
Covers: REQ-004, REQ-005, REQ-006, REQ-007   Judgement: merged from REQ-004, REQ-005, REQ-006, REQ-007

**Narrative.** As a seller, I want a title, 5 bullets and a description written for each channel in my brand's voice, so that I stop rewriting every listing per platform.

**Why it matters.** B1: this replaces the per-channel rewrite that takes a month today; B4: one source of attributes and voice keeps channels consistent.

**From the PRD.**
- REQ-004: "The system generates a title for each product for each enabled channel."
- REQ-005: "The system generates 5 bullet points for each product for each enabled channel."
- REQ-006: "The system generates a description for each product for each enabled channel."
- REQ-007: "The system writes generated listing text in the brand's own voice."

**Preconditions.**
- Detection is done for the product (US-00-002).
- At least one channel is enabled (US-00-005).

**Acceptance criteria.**

- AC-US-00-003-1. Given a product with detected attributes and two enabled channels, when generation runs, then the product has one listing per enabled channel, each with a title.
  Covers: REQ-004
- AC-US-00-003-2. Given generation completes for a listing, when the seller opens it, then it shows exactly 5 bullet points.
  Covers: REQ-005
- AC-US-00-003-3. Given generation completes for a listing, when the seller opens it, then it shows a non-empty description.
  Covers: REQ-006
- AC-US-00-003-4. Given a brand with a saved voice note, when generation runs for that brand's products, then the voice note is included in every generation prompt, visible in the recorded prompt for the call.
  Covers: REQ-007
- AC-US-00-003-5. Given a brand with no voice note, when the seller starts generation, then the seller is asked to add one or confirm a neutral default before generation starts.
  Covers: REQ-007
- AC-US-00-003-6. Given a batch of products, when generation runs, then it runs as a background job and the seller sees per-product progress without keeping the page open.
  Covers: REQ-004

**Not in this story.**
- Checking the generated text against channel rules (US-00-004).
- Regenerating a single field (US-00-008).
- Stopping generation when the budget limit is passed (US-00-012).

**Depends on.**
- US-00-002: detected attributes feed the prompt.
- US-00-005: the enabled channels and their field limits.
- US-00-011: the gateway that records cost per call.

**Assumptions.**
- Brand voice comes from a short voice note per brand (Q-002).
- Background jobs use `brg_jobs` (Q-017).
- A full 300-SKU run fits under the budget (Q-018).

**Tasks.** not written in this run

## EP-02 Get only compliant listings

Goal: every listing shows whether it meets its channel's rules, and channels are added by configuration.
Covers: REQ-008, REQ-009, REQ-010, REQ-017, REQ-018, REQ-019

### US-00-005 Set up channels and their rules as configuration

Epic: EP-02   Priority: Must   Points: TBD (estimate)
Persona: Catalogue reviewer, group 00   Ticket: unassigned
Covers: REQ-017, REQ-018   Judgement: merged from REQ-017, REQ-018

**Narrative.** As a catalogue reviewer, I want each channel and its rules held as configuration, so that the own-website and Amazon-style channels work and a new channel needs no code.

**Why it matters.** B2: the rules engine can only check what the channel configuration says; no listing can be validated until the two channels exist.

**From the PRD.**
- REQ-017: "The system stores channel rules as data, so a channel is added through configuration without code changes."
- REQ-018: "The system supports two channels: the brand's own website and an Amazon-style marketplace."

**Preconditions.**
- none

**Acceptance criteria.**

- AC-US-00-005-1. Given a fresh install, when the reviewer lists channels, then "Own website" and "Amazon-style marketplace" are present and enabled.
  Covers: REQ-018
- AC-US-00-005-2. Given a channel configuration, when it is loaded, then it holds a title length limit, a banned word list, the required attributes and the export column headers for that channel.
  Covers: REQ-017
- AC-US-00-005-3. Given a channel configuration with a missing or invalid field, when the system loads it, then the load fails with a message naming the channel and the field, and no channel from that file is enabled.
  Covers: REQ-017
- AC-US-00-005-4. Given a changed title limit in a channel's configuration, when the configuration is reloaded, then the next validation uses the new limit with no code change.
  Covers: REQ-017

**Not in this story.**
- The rule checks themselves (US-00-004).
- The Flipkart-style channel (US-00-006).
- A screen for editing rules; configuration is a file in the repository (inferred:, raise via prd if wanted).

**Depends on.**
- none

**Assumptions.**
- The team drafts the rule values for sign-off (Q-003).
- Export headers live in channel configuration (Q-015).

**Tasks.** not written in this run

### US-00-004 Validate every listing against its channel's rules

Epic: EP-02   Priority: Must   Points: TBD (estimate)
Persona: Catalogue reviewer, group 00   Ticket: unassigned
Covers: REQ-008, REQ-009, REQ-010   Judgement: merged from REQ-008, REQ-009, REQ-010

**Narrative.** As a catalogue reviewer, I want every listing checked by code against its channel's rules with each failure shown, so that I fix real problems instead of hunting for them.

**Why it matters.** B2: the share of listings passing the rules is a success measure, and it is computed from these checks.

**From the PRD.**
- REQ-008: "The system validates every generated listing against each channel's rules using a code-based rules engine, so AI output is never accepted without that check."
- REQ-009: "The system shows each rule failure on the listing, per channel."
- REQ-010: "The channel rules cover title length limits, banned words and required attributes."

**Preconditions.**
- The listing has been generated (US-00-003) and its channel is configured (US-00-005).

**Acceptance criteria.**

- AC-US-00-004-1. Given a newly generated listing, when generation finishes, then the rules engine validates it before it appears in the review grid, and the listing shows a status of passing or failing.
  Covers: REQ-008
- AC-US-00-004-2. Given a listing whose title is longer than the channel's limit, when it is validated, then a failure names the rule, the limit and the actual length.
  Covers: REQ-010, REQ-009
- AC-US-00-004-3. Given a listing containing a word on the channel's banned list, when it is validated, then a failure names the word and the field it is in.
  Covers: REQ-010, REQ-009
- AC-US-00-004-4. Given a listing missing an attribute the channel requires, when it is validated, then a failure names the missing attribute.
  Covers: REQ-010, REQ-009
- AC-US-00-004-5. Given a product with listings on two channels where only one fails, when the reviewer opens the product, then the failure shows against that channel only.
  Covers: REQ-009
- AC-US-00-004-6. Given a listing changed by an edit or a regeneration, when the change is saved, then it is validated again and its status and failures are updated.
  Covers: REQ-008
- AC-US-00-004-7. Given the same listing and rules, when validation runs twice, then the results are identical and no AI call is made.
  Covers: REQ-008

**Not in this story.**
- Approving a listing (US-00-009).
- The rule values themselves (Q-003).
- AI judging compliance (PRD constraint: rule validation is code, not AI).

**Depends on.**
- US-00-003: listings to validate.
- US-00-005: the rules to validate against.

**Assumptions.**
- Edited and regenerated listings are validated again (Q-010).
- A failing listing cannot be approved (Q-009).

**Tasks.** not written in this run

### US-00-006 Add a Flipkart-style channel through configuration only

Epic: EP-02   Priority: Could   Points: TBD (estimate)
Persona: Seller, group 00   Ticket: unassigned
Covers: REQ-019   Judgement: story (stretch)

**Narrative.** As a seller, I want a Flipkart-style channel added from configuration alone, so that I can see a new marketplace costs settings, not engineering.

**Why it matters.** B1: a third channel without code is the proof that each new marketplace does not restart the month of rewriting.

**From the PRD.**
- REQ-019: "The system supports a Flipkart-style channel added through configuration only."

**Preconditions.**
- Channel configuration and validation work for the two core channels (US-00-005, US-00-004).

**Acceptance criteria.**

- AC-US-00-006-1. Given a new Flipkart-style channel file and no code change, when configuration is reloaded, then the channel is listed and enabled.
  Covers: REQ-019
- AC-US-00-006-2. Given the Flipkart-style channel is enabled, when generation runs, then each product gets a Flipkart-style listing validated against that channel's rules.
  Covers: REQ-019
- AC-US-00-006-3. Given approved Flipkart-style listings, when the reviewer exports, then a separate Flipkart-style CSV is produced.
  Covers: REQ-019
- AC-US-00-006-4. Given the change that added the channel, when it is reviewed, then it touches configuration files only.
  Covers: REQ-019

**Not in this story.**
- Any channel beyond Flipkart-style (PRD non-goal: a third channel; Q-007).
- Publishing to Flipkart (PRD non-goal).

**Depends on.**
- US-00-005: configuration format.
- US-00-004: validation driven by configuration.

**Assumptions.**
- Flipkart-style is stretch only, built purely through configuration (Q-007).

**Tasks.** not written in this run

## EP-03 Review and approve listings

Goal: a reviewer fixes and approves a whole launch's listings from one grid.
Covers: REQ-011, REQ-012, REQ-013

### US-00-007 Edit listings in a review grid

Epic: EP-03   Priority: Must   Points: TBD (estimate)
Persona: Catalogue reviewer, group 00   Ticket: unassigned
Covers: REQ-011   Judgement: story

**Narrative.** As a catalogue reviewer, I want to see and edit every listing in one grid, so that I can fix a launch without opening products one at a time.

**Why it matters.** B1: review is the step a human must do; a grid keeps it to days, not weeks.

**From the PRD.**
- REQ-011: "The system lets a reviewer edit listing fields in a grid."

**Preconditions.**
- Listings have been generated and validated (US-00-004).
- The user is in reviewer mode (Q-014).

**Acceptance criteria.**

- AC-US-00-007-1. Given generated listings, when the reviewer opens the grid, then each row is one product and channel, with title, bullets, description, attributes, rule status and approval status.
  Covers: REQ-011
- AC-US-00-007-2. Given a listing in the grid, when the reviewer edits a text field and saves, then the new value is stored and shown after a page reload.
  Covers: REQ-011
- AC-US-00-007-3. Given a product in the grid, when the reviewer corrects a detected attribute and saves, then the attribute is stored and every listing of that product is validated again.
  Covers: REQ-011
- AC-US-00-007-4. Given an approved listing, when the reviewer edits any field, then its approval is cleared and it must be approved again.
  Covers: REQ-011
- AC-US-00-007-5. Given the grid, when the reviewer filters by channel or by rule status, then only matching rows are shown.
  Covers: REQ-011

**Not in this story.**
- Regenerating a field with AI (US-00-008).
- Approving listings (US-00-009).
- Sign-in and permissions (Q-014).

**Depends on.**
- US-00-004: rule status to show and re-validation on save.

**Assumptions.**
- Detected attributes are editable (Q-016).
- An edit clears approval (Q-022).
- No sign-in; a reviewer mode switch (Q-014).

**Tasks.** not written in this run

### US-00-008 Regenerate one field with an instruction

Epic: EP-03   Priority: Must   Points: TBD (estimate)
Persona: Catalogue reviewer, group 00   Ticket: unassigned
Covers: REQ-013   Judgement: story

**Narrative.** As a catalogue reviewer, I want to regenerate one field with an instruction like "shorter, mention cotton", so that I fix a weak line without rewriting it or redoing the listing.

**Why it matters.** B1: a targeted fix takes seconds where a rewrite takes minutes per SKU.

**From the PRD.**
- REQ-013: "The system lets a reviewer regenerate a single field of a listing with a free-text instruction."

**Preconditions.**
- The listing exists in the grid (US-00-007).

**Acceptance criteria.**

- AC-US-00-008-1. Given a listing, when the reviewer regenerates its title with an instruction, then only the title changes and the bullets and description are unchanged.
  Covers: REQ-013
- AC-US-00-008-2. Given a listing, when the reviewer regenerates one bullet, then only that bullet changes and the listing still has 5 bullets.
  Covers: REQ-013
- AC-US-00-008-3. Given a regeneration, when it finishes, then the instruction is included in the prompt and the listing is validated again.
  Covers: REQ-013
- AC-US-00-008-4. Given a regeneration, when it finishes, then its cost is added to the product's AI cost.
  Covers: REQ-013
- AC-US-00-008-5. Given an approved listing, when a field is regenerated, then its approval is cleared.
  Covers: REQ-013

**Not in this story.**
- Regenerating a whole listing or a whole batch (US-00-003).
- Undoing a regeneration (not in the PRD; raise via prd if wanted).

**Depends on.**
- US-00-007: the grid the action lives in.

**Assumptions.**
- Regeneration clears approval (Q-022).
- Regenerated listings are validated again (Q-010).

**Tasks.** not written in this run

### US-00-009 Approve listings in bulk

Epic: EP-03   Priority: Must   Points: TBD (estimate)
Persona: Catalogue reviewer, group 00   Ticket: unassigned
Covers: REQ-012   Judgement: story

**Narrative.** As a catalogue reviewer, I want to approve many listings in one action, so that signing off 300 SKUs takes minutes once I have checked them.

**Why it matters.** B1: approval is the human gate before export; one click per listing would make it the slowest step.

**From the PRD.**
- REQ-012: "The system lets a reviewer approve many listings in one action."

**Preconditions.**
- Listings are validated (US-00-004) and shown in the grid (US-00-007).

**Acceptance criteria.**

- AC-US-00-009-1. Given several passing listings selected in the grid, when the reviewer approves them, then all of them show as approved.
  Covers: REQ-012
- AC-US-00-009-2. Given a selection that includes failing listings, when the reviewer approves, then only the passing ones are approved and the reviewer is told how many were skipped and why.
  Covers: REQ-012
- AC-US-00-009-3. Given a failing listing, when an approve request for it reaches the service directly, then the service refuses it.
  Covers: REQ-012
- AC-US-00-009-4. Given an approved listing, when the reviewer views it, then it shows when it was approved.
  Covers: REQ-012
- AC-US-00-009-5. Given seller mode, when the user views the grid, then no approve action is offered.
  Covers: REQ-012

**Not in this story.**
- Exporting approved listings (US-00-010).
- Overriding a rule failure (Q-009).

**Depends on.**
- US-00-004: pass or fail status.
- US-00-007: the grid and selection.

**Assumptions.**
- Failing listings cannot be approved (Q-009).
- Approval is per listing, that is per product and channel (Q-023).
- Approval is hidden in seller mode, not enforced by sign-in (Q-014).

**Tasks.** not written in this run

## EP-04 Export approved listings

Goal: a reviewer downloads one ready-to-upload CSV per channel containing only approved listings.
Covers: REQ-014, REQ-015

### US-00-010 Export approved listings as one CSV per channel

Epic: EP-04   Priority: Must   Points: TBD (estimate)
Persona: Catalogue reviewer, group 00   Ticket: unassigned
Covers: REQ-014, REQ-015   Judgement: merged from REQ-014, REQ-015

**Narrative.** As a catalogue reviewer, I want to download one CSV per channel holding only approved listings, so that I can upload each file to its marketplace.

**Why it matters.** B1: the export is the end of the launch inside Catalift; until it ships, nothing leaves the system.

**From the PRD.**
- REQ-014: "The system exports approved listings as one CSV file per channel."
- REQ-015: "The system excludes listings a reviewer has not approved from every export."

**Preconditions.**
- At least one listing is approved (US-00-009).

**Acceptance criteria.**

- AC-US-00-010-1. Given approved listings on two channels, when the reviewer exports, then two CSV files are produced, one per channel.
  Covers: REQ-014
- AC-US-00-010-2. Given a channel's export, when the file is opened, then its headers are the export columns from that channel's configuration and each row is one approved listing.
  Covers: REQ-014
- AC-US-00-010-3. Given a mix of approved and unapproved listings, when the reviewer exports, then no unapproved listing appears in any file.
  Covers: REQ-015
- AC-US-00-010-4. Given an approved listing that was edited and lost its approval, when the reviewer exports, then it is not in the file.
  Covers: REQ-015
- AC-US-00-010-5. Given a channel with no approved listings, when the reviewer exports, then no file is produced for it and the reviewer is told why.
  Covers: REQ-014

**Not in this story.**
- Publishing to marketplaces (PRD non-goal).
- Matching a marketplace's own bulk-upload template (Q-015).

**Depends on.**
- US-00-009: approvals.

**Assumptions.**
- Export columns are SKU, title, bullet 1 to 5, description and attributes, with headers from configuration (Q-015).

**Tasks.** not written in this run

## EP-05 Keep AI cost visible and capped

Goal: a seller sees what each product cost in AI calls, and spending stops before the budget is exceeded.
Covers: REQ-016, REQ-020, REQ-021

### US-00-011 See the AI cost of each product

Epic: EP-05   Priority: Must   Points: TBD (estimate)
Persona: Seller, group 00   Ticket: unassigned
Covers: REQ-016, REQ-020   Judgement: merged from REQ-016, REQ-020

**Narrative.** As a seller, I want each product to show what its AI calls cost, so that I know what every listing costs me.

**Why it matters.** B5: cost per listing is the visibility the brief promises; every other AI story records cost through this gateway.

**From the PRD.**
- REQ-016: "The system shows the AI cost of each product."
- REQ-020: "The system records the token count and cost of every AI call."

**Preconditions.**
- none

**Acceptance criteria.**

- AC-US-00-011-1. Given any AI call, when it completes, then a record holds the product, the purpose, input and output tokens and the cost in USD.
  Covers: REQ-020
- AC-US-00-011-2. Given an AI call that fails, when it returns, then it is still recorded with its tokens and cost, if any.
  Covers: REQ-020
- AC-US-00-011-3. Given a product with detection, generation and regeneration calls, when the seller views it, then its AI cost is the sum of all its calls in USD.
  Covers: REQ-016
- AC-US-00-011-4. Given the product list, when the seller views it, then each product shows its AI cost and the batch shows a total.
  Covers: REQ-016

**Not in this story.**
- Blocking calls at the budget limit (US-00-012).
- Billing sellers (not in the PRD).

**Depends on.**
- none

**Assumptions.**
- Cost includes every call for the product, regenerations included (Q-011).
- The gateway is `brg_llm_gateway` (Q-017).

**Tasks.** not written in this run

### US-00-012 Stop AI calls once the budget limit is passed

Epic: EP-05   Priority: Must   Points: TBD (estimate)
Persona: Seller, group 00   Ticket: unassigned
Covers: REQ-021   Judgement: story

**Narrative.** As a seller, I want AI calls blocked once spending passes USD 8, so that the USD 10 budget is never exceeded.

**Why it matters.** B5: cost stays known and bounded; without the block, one runaway batch spends the whole budget.

**From the PRD.**
- REQ-021: "The system blocks further AI calls once total AI spending passes USD 8."

**Preconditions.**
- Cost is recorded for every call (US-00-011).

**Acceptance criteria.**

- AC-US-00-012-1. Given total recorded spend above USD 8, when any AI call is requested, then the gateway refuses it without calling the provider.
  Covers: REQ-021
- AC-US-00-012-2. Given a batch running when spend passes USD 8, when the next call is refused, then completed products keep their results and remaining products are marked "stopped: budget".
  Covers: REQ-021
- AC-US-00-012-3. Given spend is above the limit, when the seller views the product list, then a banner shows total spend and that AI calls are blocked.
  Covers: REQ-021
- AC-US-00-012-4. Given the test suite, when it runs, then it replays recorded AI responses and adds nothing to recorded spend.
  Covers: REQ-021

**Not in this story.**
- Raising or resetting the limit from the UI (Q-004).
- Per-batch or monthly limits (Q-004).

**Depends on.**
- US-00-011: recorded spend.

**Assumptions.**
- The limit is lifetime spend with a hard stop past USD 8 (Q-004).
- Mid-batch behaviour as in AC 2 (Q-021).

**Tasks.** not written in this run
