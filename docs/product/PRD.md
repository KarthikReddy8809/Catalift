# PRD: Catalift

Source: product briefing pasted in chat ("Catalift: Product Briefing", 62 lines) Normalised: 2026-10-01
Owner: unconfirmed: Tracker epic: unconfirmed:

Product name: the brief uses "Catalift" and invites a different name; the repository folder is "Catalift". See Q-001.

## 1. Problem

Sellers who list on their own website, Amazon and Flipkart rewrite every listing for each platform, because each platform has its own title limits, banned words and required attributes (brief, "The problem"). For a brand launching 300 SKUs a season this takes about a month, and the resulting listings are inconsistent and rank poorly in search (brief, "The problem").

## 2. Business objectives

| Id  | Objective                                                                          | Target (measurable)                                                                        | Source                                                           |
| --- | ---------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------ | ---------------------------------------------------------------- |
| B1  | A seasonal launch of 300 SKUs is ready to list in far less time than today's month | target: unconfirmed (brief claims "a week"; see Q-008)                                     | "a 300-SKU launch goes live in a week instead of a month"        |
| B2  | Generated listings meet each marketplace's rules                                   | target: unconfirmed (measured as share of generated listings passing the rules; see Q-012) | "What share of generated listings pass the marketplace rules"    |
| B3  | Product attributes are detected accurately from photos                             | target: unconfirmed (measured on 30 labelled products; see Q-012, Q-013)                   | "How accurately attributes are detected on 30 labelled products" |
| B4  | Listings are consistent across channels and rank better in search                  | target: unconfirmed (no measure in the brief)                                              | "the results are inconsistent and rank poorly in search"         |
| B5  | Sellers know what each listing costs to produce                                    | target: unconfirmed                                                                        | "Cost is visible ... sellers know what every listing costs"      |

## 3. Non-goals

- Publishing listings directly to marketplaces (brief, Scope: Out).
- Image editing (brief, Scope: Out).
- A third channel (brief, Scope: Out), except the Flipkart-style stretch channel, see REQ-019 and Q-007.

## 4. Personas

| Persona            | Group       | Who they are                                       | What they need                                                            | Source                                                         |
| ------------------ | ----------- | -------------------------------------------------- | ------------------------------------------------------------------------- | -------------------------------------------------------------- |
| Seller             | 00 end user | D2C brand or marketplace seller launching products | upload products in bulk and receive ready-to-publish listings per channel | "Seller: uploads the products."                                |
| Catalogue reviewer | 00 end user | person who checks listing quality before export    | check, edit and approve listings, and fix single fields quickly           | "Catalogue reviewer: checks, edits and approves the listings." |

## 5. Requirement statements

One testable statement per id. Ids are never reused or renumbered.

| Id      | Statement                                                                                                                                                     | Persona            | Source                                                            | Flags                                 |
| ------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------ | ----------------------------------------------------------------- | ------------------------------------- |
| REQ-001 | The system accepts a bulk CSV upload with one row per product carrying SKU, category, brand and price.                                                        | Seller             | Solution 1: "a bulk CSV (SKU, category, brand, price)"            | none                                  |
| REQ-002 | The system accepts product image uploads and associates each image with a product from the CSV.                                                               | Seller             | Solution 1: "plus product images"                                 | ambiguous: Q-005                      |
| REQ-003 | The system detects colour, pattern, sleeve, neckline and fit from each product photo using AI.                                                                | Seller             | Solution 2                                                        | none (scope see Q-006, Q-016)         |
| REQ-004 | The system generates a title for each product for each enabled channel.                                                                                       | Seller             | Solution 3                                                        | none                                  |
| REQ-005 | The system generates 5 bullet points for each product for each enabled channel.                                                                               | Seller             | Solution 3                                                        | none                                  |
| REQ-006 | The system generates a description for each product for each enabled channel.                                                                                 | Seller             | Solution 3                                                        | none                                  |
| REQ-007 | The system writes generated listing text in the brand's own voice.                                                                                            | Seller             | Solution 3: "in the brand's own voice"                            | ambiguous: Q-002                      |
| REQ-008 | The system validates every generated listing against each channel's rules using a code-based rules engine, so AI output is never accepted without that check. | Catalogue reviewer | Solution 4; "Compliant by design"                                 | none (re-check after edits see Q-010) |
| REQ-009 | The system shows each rule failure on the listing, per channel.                                                                                               | Catalogue reviewer | Solution 4: "flags errors per channel"                            | none (effect on approval see Q-009)   |
| REQ-010 | The channel rules cover title length limits, banned words and required attributes.                                                                            | Catalogue reviewer | The problem: "title limits, banned words and required attributes" | ambiguous: Q-003                      |
| REQ-011 | The system lets a reviewer edit listing fields in a grid.                                                                                                     | Catalogue reviewer | Solution 5                                                        | none                                  |
| REQ-012 | The system lets a reviewer approve many listings in one action.                                                                                               | Catalogue reviewer | Solution 5: "bulk-approve"                                        | none                                  |
| REQ-013 | The system lets a reviewer regenerate a single field of a listing with a free-text instruction.                                                               | Catalogue reviewer | Solution 5                                                        | none                                  |
| REQ-014 | The system exports approved listings as one CSV file per channel.                                                                                             | Catalogue reviewer | Solution 6                                                        | none (columns see Q-015)              |
| REQ-015 | The system excludes listings a reviewer has not approved from every export.                                                                                   | Catalogue reviewer | "nothing is exported until a reviewer approves it"                | none                                  |
| REQ-016 | The system shows the AI cost of each product.                                                                                                                 | Seller             | "each product shows its AI cost"                                  | ambiguous: Q-011                      |
| REQ-017 | The system stores channel rules as data, so a channel is added through configuration without code changes.                                                    | Catalogue reviewer | "channel rules are stored as data"                                | none                                  |
| REQ-018 | The system supports two channels: the brand's own website and an Amazon-style marketplace.                                                                    | Seller             | Scope: In                                                         | none (rule content see Q-003)         |
| REQ-019 | The system supports a Flipkart-style channel added through configuration only.                                                                                | Seller             | Scope: Stretch                                                    | stretch; see Q-007                    |
| REQ-020 | The system records the token count and cost of every AI call.                                                                                                 | Seller             | Tech: "logs tokens and cost"                                      | none                                  |
| REQ-021 | The system blocks further AI calls once total AI spending passes USD 8.                                                                                       | Seller             | Tech: "calls blocked once spending passes USD 8"                  | ambiguous: Q-004                      |

## 6. Constraints

- AI model: `claude-haiku-4-5` reached through OpenRouter (brief, Tech and AI).
- Every AI call goes through one gateway; the brief names Bearing's `brg_llm_gateway` (brief, Tech and AI).
- Budget cap of USD 10 for AI spend, with calls blocked past USD 8 (brief, Tech and AI; see Q-004, Q-018).
- Storage: Postgres in Docker, product images on local storage (brief, Tech and AI).
- Tests replay recorded AI responses, so the test suite makes no paid AI calls (brief, Tech and AI).
- Built on Bearing's existing components `brg_llm_gateway`, `brg_jobs` and `brg_design_variants` (brief, Tech and AI; location see Q-017).
- Rule validation is code, not AI (brief, "Compliant by design").
- Output is CSV per channel; no marketplace API integration (brief, Scope).
- Demo uses simple drawn product images; fallback is more varied patterns and textures, with a plain statement that real photos come later (brief, Risk and fallback).
- inferred: detection attributes (sleeve, neckline, fit) imply apparel products only (see Q-006).

## 7. Open questions

18 entries in docs/product/questions.md: 18 open, 14 need your confirmation (Q-001, Q-002, Q-003, Q-004, Q-005, Q-008, Q-011, Q-012, Q-013, Q-014, Q-015, Q-016, Q-017, Q-018).

## 8. Could not extract

- Owner and tracker epic: not in the input; the product owner can supply them.
- Measurable targets for B1 to B5: the brief gives measures and claims but no target values; the product owner can set them (Q-008, Q-012).
- Channel rule content (actual limits, banned word lists, required attributes per channel): the product owner or a marketplace specialist (Q-003).

## 9. Glossary

| Term                          | Meaning                                                                                                                    | Source     |
| ----------------------------- | -------------------------------------------------------------------------------------------------------------------------- | ---------- |
| SKU                           | one product variant, one row in the upload CSV                                                                             | Solution 1 |
| Channel                       | a place a listing is sold: the brand's own website, an Amazon-style marketplace, or the stretch Flipkart-style marketplace | Scope      |
| Listing                       | per product and per channel: a title, 5 bullet points and a description                                                    | Solution 3 |
| D2C brand                     | a brand that sells direct to consumers, including on its own website                                                       | What it is |
| Rules engine                  | code that validates a listing against a channel's rules and reports errors per channel                                     | Solution 4 |
| Amazon-style / Flipkart-style | a channel modelled on that marketplace's rules, not an integration with it                                                 | Scope      |
