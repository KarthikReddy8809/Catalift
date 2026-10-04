# System design tenets: Catalift

Rules this team holds itself to on this project. Each is here because
somebody could plausibly do the opposite, and a reviewer can point at a
breach.

## 1. A domain reads only its own tables

**Each domain (auth, catalogue, listings, export, cost, jobs) queries only the tables it owns; anything else goes through the owning domain's Go interface.**

The backend is one binary by choice (HLD section 1), and the only thing that keeps the domain APIs separable later is that no SQL crosses a domain line.

_A breach looks like:_ an MR adds a sqlc query in the export package that joins `listings` with `approvals` directly, instead of asking the listings domain for approved listings.

## 2. Every AI call goes through the gateway, and only the worker calls it

**No code outside the AI gateway package opens a connection to OpenRouter, and no HTTP handler calls the gateway; AI work is always a job.**

The USD 8 block, the cost ledger and test replay all live in the gateway (REQ-020, REQ-021, ADR-0004); one call around it spends money nobody sees.

_A breach looks like:_ an MR makes the regenerate endpoint call OpenRouter inline "to make it feel faster", with its own HTTP client and no cost record.

## 3. The rules engine is code and decides alone

**Whether a listing passes is computed only by the rules engine from the listing and its channel config; no AI output, prompt or reviewer flag sets rule status.**

"Compliant by design" is the product's claim (REQ-008, PRD Constraints); if a model or a checkbox can mark a listing passing, the claim is false.

_A breach looks like:_ an MR asks the model "does this listing meet Amazon rules? answer yes or no" and stores the answer as the rule status, or adds an "override" flag that approval treats as passing.

## 4. A state change and its job commit together

**A job is enqueued in the same database transaction as the state change it belongs to, and the guard a job relies on is re-read when the job runs.**

The job table exists for this (ADR-0005); enqueue after commit loses work, and a status read at enqueue time can be stale by the time the job runs.

_A breach looks like:_ an MR commits the product status update, then calls `jobs.Enqueue` in a second transaction, or a generate job trusts a `channel` value copied into its payload instead of re-reading whether the channel is still configured.

## 5. Any change to a listing clears its approval in the same write

**Every write to a listing's text or its product's attributes raises the listing version and clears approval in the same transaction; approval and export match on version.**

"Nothing is exported until a reviewer approves it" (REQ-015) only holds if an approval covers exactly the text that ships (Q-022).

_A breach looks like:_ an MR adds a "fix typos" bulk action that updates titles without touching version or approval, so the edited text exports under an approval given to the old text.

## 6. Roles are checked by the API on every route

**Each handler declares the roles allowed, and the router refuses the request before the handler runs; hiding a button is never the check.**

ADR-0006 exists because the public VM spends a real budget and only reviewers may approve and export (conflict 1).

_A breach looks like:_ an MR adds `POST /v1/exports/{id}/resend` with no role declaration, relying on the web app only showing the button to reviewers.

## 7. The contract changes before the code

**Every new or changed endpoint is in `api/openapi.yaml` in the same MR, and the web app uses only the generated client.**

ADR-0007 makes the spec the single source of truth, and the web app's types are generated from it.

_A breach looks like:_ an MR adds a `bulkRegenerate` handler in Go and a hand-written axios call in the web app, with no spec change, so the generated types never learn about it.
