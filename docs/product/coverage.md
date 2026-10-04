# Coverage: PRD statements to stories

PRD: docs/product/PRD.md   Backlog: docs/product/backlog.md   Built: 2026-10-01

No code, config or tests exist in this repository yet, so no statement could be checked against an enforced rule; every rule below is new work.

## Matrix

| REQ | Statement (short) | Judgement | Why | Covered by | AC ids |
| --- | --- | --- | --- | --- | --- |
| REQ-001 | Bulk CSV upload with SKU, category, brand, price | story | a seller can load a launch for the first time when it lands | US-00-001 | AC-US-00-001-1, AC-US-00-001-2 |
| REQ-002 | Image upload associated with CSV products | criterion-of US-00-001 | images are part of the same upload and are useless without the products they attach to | US-00-001 | AC-US-00-001-3, AC-US-00-001-4, AC-US-00-001-5 |
| REQ-003 | AI detects colour, pattern, sleeve, neckline, fit | story | the seller gets attributes without typing, a new capability on its own | US-00-002 | AC-US-00-002-1, AC-US-00-002-2, AC-US-00-002-3, AC-US-00-002-4 |
| REQ-004 | Title per product per channel | story | it seeds generation, the first point where a seller gets a listing | US-00-003 | AC-US-00-003-1, AC-US-00-003-6 |
| REQ-005 | 5 bullets per product per channel | criterion-of US-00-003 | bullets come from the same generation run; shipping them alone gives no usable listing | US-00-003 | AC-US-00-003-2 |
| REQ-006 | Description per product per channel | criterion-of US-00-003 | same generation run as the title and bullets | US-00-003 | AC-US-00-003-3 |
| REQ-007 | Text in the brand's own voice | criterion-of US-00-003 | a quality of the generated text, not something a user does on its own | US-00-003 | AC-US-00-003-4, AC-US-00-003-5 |
| REQ-008 | Code rules engine validates every listing | story | the reviewer gets a compliance check they do not have today | US-00-004 | AC-US-00-004-1, AC-US-00-004-6, AC-US-00-004-7 |
| REQ-009 | Each rule failure shown per channel | criterion-of US-00-004 | how validation results are shown, part of the same check | US-00-004 | AC-US-00-004-2, AC-US-00-004-3, AC-US-00-004-4, AC-US-00-004-5 |
| REQ-010 | Rules cover title limits, banned words, required attributes | criterion-of US-00-004 | names the rule types the engine checks | US-00-004 | AC-US-00-004-2, AC-US-00-004-3, AC-US-00-004-4 |
| REQ-011 | Reviewer edits fields in a grid | story | the reviewer can fix listings in one place for the first time | US-00-007 | AC-US-00-007-1 to AC-US-00-007-5 |
| REQ-012 | Reviewer approves many listings at once | story | approval is the human gate and ships on its own | US-00-009 | AC-US-00-009-1 to AC-US-00-009-5 |
| REQ-013 | Regenerate one field with an instruction | story | a separate reviewer action with its own AI call and cost | US-00-008 | AC-US-00-008-1 to AC-US-00-008-5 |
| REQ-014 | One CSV per channel of approved listings | story | the reviewer gets files to upload, the end of the flow | US-00-010 | AC-US-00-010-1, AC-US-00-010-2, AC-US-00-010-5 |
| REQ-015 | Unapproved listings never exported | criterion-of US-00-010 | a condition on what the export contains | US-00-010 | AC-US-00-010-3, AC-US-00-010-4 |
| REQ-016 | AI cost shown per product | story | the seller sees cost per product for the first time | US-00-011 | AC-US-00-011-3, AC-US-00-011-4 |
| REQ-017 | Channel rules stored as data | story | the configuration format every channel depends on; merged with REQ-018 since the two channels are its first use | US-00-005 | AC-US-00-005-2, AC-US-00-005-3, AC-US-00-005-4 |
| REQ-018 | Two channels: own website, Amazon-style | criterion-of US-00-005 | the two channels are the configuration's initial content | US-00-005 | AC-US-00-005-1 |
| REQ-019 | Flipkart-style channel by configuration only | story | stretch, ships on its own once the core channels work (Q-007) | US-00-006 | AC-US-00-006-1 to AC-US-00-006-4 |
| REQ-020 | Tokens and cost recorded for every AI call | criterion-of US-00-011 | the per-call record that the per-product cost sums | US-00-011 | AC-US-00-011-1, AC-US-00-011-2 |
| REQ-021 | AI calls blocked past USD 8 | story | a distinct protective behaviour the seller relies on | US-00-012 | AC-US-00-012-1 to AC-US-00-012-4 |

## Gaps

| REQ | Why uncovered | Proposed action |
| --- | --- | --- |
| none | | |

## Orphan stories

| Story | Reason it exists | Action |
| --- | --- | --- |
| none | a measurement story for B2 and B3 was drafted, then raised as Q-024 instead, because a story must trace to a REQ | answer Q-024, then add the REQ via prd |

## Counts

stories-coverage: 21 REQ from docs/product/PRD.md (0 withdrawn), 21 covered, 0 out of scope, 0 gaps, 12 stories, 58 AC, 0 orphans, 0 problems
Verdict: covered
