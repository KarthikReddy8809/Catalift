# TODOS

## Architecture

### Supersede ADR-0005, 0008, 0009 and 0010 to match the eng review

**What:** Write superseding ADRs through `tech-decision` for ADR-0005 (retry wording, job key), ADR-0008 (data disk, CI bootstrap, resource count), ADR-0009 (one repository) and ADR-0010 (seven alerts, backup alert mechanism).

**Why:** Four accepted ADRs still describe choices the 2026-10-04 eng review changed; builders read ADRs first and would rebuild the old design.

**Context:** The answers are D4, D9, D10, D11, D12, D14, D17 and D18 in the Decision ledger at the end of docs/design/catalift-hld.md. Accepted ADRs are never edited in place; each new ADR names the one it supersedes, and the old one is marked superseded only once the new one is accepted. Update docs/architecture/decisions.md and docs/decisions.md with the new rows.

**Effort:** S
**Priority:** P1
**Depends on:** None

### Reconcile "possibly charged" AI calls to their real cost

**What:** Design a reconcile step that looks up the actual cost of each "possibly charged" AI call by its OpenRouter generation id and corrects the cost ledger; calls with no id keep their reserved estimate.

**Why:** Timed-out or crash-interrupted calls are counted at an estimate, so cost per product (B5) is approximate and the USD 8 block can trip early.

**Context:** HLD sections 6 and 7 name monthly reconciliation without a mechanism (critic v1 "Not said"). Whether OpenRouter's generation lookup returns the cost is an open assumption in HLD section 17. Start by checking that lookup against one real call.

**Effort:** M
**Priority:** P2
**Depends on:** Confirming OpenRouter's generation cost lookup

### Define replay mode's ledger rows and missing-recording behaviour

**What:** In the AI gateway LLD, decide whether replayed calls write cost-ledger rows and what happens when no recording exists for a request.

**Why:** Phase 1 runs `AI_MODE=replay` on the deployed database; fake spend could pollute the USD 8 block and seller costs, and a missing recording that falls through to a live call would spend real money.

**Context:** The eng review (D27, 2026-10-04) recommended: replay rows marked `replay` with cost 0, excluded from the block and seller views, and a missing recording fails the job "no recording" without calling OpenRouter. The owner chose to decide this in the LLD instead. Must be settled before phase 1 deploys.

**Effort:** S
**Priority:** P2
**Depends on:** None; blocks HLD section 12 phase 1

## Product

### Bring the backlog and question register in line with the eng review

**What:** Run `backlog` (and `prd` for the Q entries) to amend AC-US-00-009-5, the US-00-007 and US-00-009 assumptions, AC-US-00-012-1 and AC-US-00-012-3, and the decisions of Q-005, Q-014, Q-017 and Q-020; then rerun the coverage gate.

**Why:** Stories and tests are written from docs/product/; four points there now contradict the approved HLD (real sign-in instead of seller mode, separator-based image matching, the stored budget-blocked state, and rejecting an existing SKU).

**Context:** The exact edits are listed under "What now has to change to match" in docs/architecture/decisions.md and in answers D13, D20 and D21 of the Decision ledger in docs/design/catalift-hld.md. Keep every story, AC and Q id; never renumber.

**Effort:** S
**Priority:** P1
**Depends on:** None

### Let sellers change a product's category, brand and price after upload

**What:** Propose a REQ through `prd` and a story through `backlog` for editing a product's category, brand and price; the edit bumps that product's listing versions and clears their approvals.

**Why:** The eng review (D21) made re-uploading an existing SKU an error, which removed the only way to correct these fields.

**Context:** Not needed for the demo, where the team prices the drawn products. Real sellers change prices often. Product fields live in the catalogue area; listings live in the listings area, so the LLD decides how the version bump and approval clear stay in step (see tenet 1 and D5).

**Effort:** M
**Priority:** P3
**Depends on:** None

## Infrastructure

### Estimate monthly Google Cloud cost and set a billing alert

**What:** Estimate the monthly cost of the VM, boot and data disks, snapshots, buckets, Artifact Registry and Cloud Monitoring with the Google Cloud pricing calculator, and set a billing budget alert on the project.

**Why:** AI spend is capped at USD 10, but infrastructure spend has no estimate and no alert; a forgotten resource or piling snapshots could cost more than the AI budget unnoticed.

**Context:** Raised by the critic (v1 "Not said") and accepted as a TODO in the eng review (D29, 2026-10-04). HLD section 8 assumes a 2 vCPU VM; final sizes come from the LLD.

**Effort:** S
**Priority:** P3
**Depends on:** Machine and disk sizes from the LLD
