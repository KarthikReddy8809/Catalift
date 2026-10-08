# ADR-0012: Reviewers send approved exports to the seller; the seller owns brand voice

- Status: Accepted
- Date: 2026-10-07
- Task: US-00-011
- Deciders: Karthik Reddy
- Area: roles and access
- Reversibility: cheap: role checks sit on four routes and the sidebar; the two new columns are additive
- Supersedes: ADR-0006 in part (who may read exports, and who edits brand voice); the rest of ADR-0006 stands

## Context

- From the request (2026-10-07): "in the export section please add the CTA that send the csv to the seller and onclick of this the CSV will be moved to the seller", after "the seller wants the csv right not the reviewer".
- The same request: "remove the existing review and the channels section and add the Brand voice section for the seller side".
- The brief asks for "Brand voice settings (tone, words to avoid)". Only a free-text voice note existed, and any role could change it.
- ADR-0006 made export a reviewer action so that the approval gate means something: "approve, regenerate, edit and export are refused for sellers".

## What else was considered

| Option | Why not | Would suit |
| --- | --- | --- |
| Reviewer exports and sends; seller downloads sent exports (chosen) | one more step for the reviewer | a sign-off that is visible to the seller as a hand-over |
| Seller exports directly | the gate still holds (only approved listings are written), but nothing marks the reviewer's hand-over | a team where the seller is trusted to pick the moment |
| Keep export reviewer-only | the seller, who publishes the listings, never gets the files | a reviewer who also publishes |

## Decision

A reviewer exports as before, then sends the export to the seller (`POST /v1/exports/{id}/send`), which records who sent it and when, once. A seller lists and downloads only sent exports; an unsent one is 404 to a seller. The reviewer keeps access to every export. Brand voice becomes tone (the existing voice note) plus words to avoid; only a seller edits it, and the rules engine flags a word to avoid in a listing as `brand_avoid_word`, re-checking the brand's listings when the list changes. The seller's sidebar is Upload, Products, Brand voice and Received files; the reviewer's is Products, Review, Export and Channels, where the reviewer edits channel rules.

## Consequences

- The approval gate is unchanged: an export only ever holds listings approved at their current version (REQ-015), and a seller still cannot approve, edit, regenerate or send.
- Changing a brand's words to avoid can clear approvals, as a channel rule edit does.
- PRD REQ-014's user becomes "Catalogue reviewer, then seller"; the next PRD revision updates it.
- Migration 00007 adds `brands.words_to_avoid` and `exports.sent_at`, `exports.sent_by`.

## Commits us to

Role checks on `GET /v1/exports`, `GET /v1/exports/{id}`, its files and `POST /v1/exports/{id}/send`; seller-only `PATCH /v1/brands/{id}`.
