# Catalift screen designs (round 1, 2026-10-04)

Screens are code: each is a presentational view in `apps/web/src/features/<feature>/components/` plus a `*.screen.tsx` with every state from fixtures. Open the gallery with the web dev server running:

- Index: http://localhost:5173/__design
- One state: `http://localhost:5173/__design/S-05?state=editing` (add `&chrome=0` to hide the state switcher, `&theme=dark` for dark)

Inventory source: the stories in `docs/product/backlog.md`, `api/openapi.yaml` and HLD v3 flows A to C. There is no flows file yet (`ux-flows` writes one), so the navigation below is a proposal.

| Screen | Job | States |
| --- | --- | --- |
| S-02 Sign in | Start a session; say why when it fails | default, loading, wrong-credentials (401), rate-limited (429), session-expired |
| S-03 Upload a launch | CSV then photos; name every row and file that did not load | empty, loading, success, images-attached, file-too-large (413), wrong-type (415), conflict (409), error |
| S-04 Products and generation | Detection, listing progress and AI cost; start or resume generation | loading, empty, success, generating, voice-note-required (422), budget-blocked (D13), error |
| S-05 Review listings | See rule results, fix, approve only what passes | loading, empty, success, filtered-empty, editing, version-conflict (409), approve-result, regeneration-superseded (D12), rules-changed (D7), seller-read-only, error |
| S-06 Export | One CSV per channel of approved listings | ready, loading, success, nothing-approved (422), error, seller-forbidden (403) |
| S-07 Channels | The rules each channel checks; say when one is off | loading, success, channel-disabled (D8), error |

Navigation (proposed): Upload, Products, Review, Export (reviewers only), Channels. Desktop shows them across the top; a phone shows them as a bottom bar. The frame also shows AI spend against the limit and the blocked-budget banner.

## Gaps raised

- **[undefined-state] API gap, owner: API spec.** The review grid shows each listing's attributes (AC-US-00-007-1), but the `Listing` schema in `api/openapi.yaml` has no attributes and no attribute revision (needed for D5 edits). The design assumes `GET /v1/listings` returns them; it is labelled in `ReviewGridView.tsx`.
- **[ambiguity] Navigation, owner: product owner.** No flows file; the order above is proposed.
- **[missing-screen] Brands, owner: product owner.** S-04 tells sellers to add a voice note "in Brands", but no Brands screen is designed. It is plain text, not a link.
- **[risk] Visual direction, owner: product owner.** The screens use the stock shadcn neutral tokens. No direction has been chosen (`design-directions`), so colour, type and density are placeholders; the shared layout's 1024 px content cap (`RootLayout.tsx`) also leaves margins at 1440.
- **[risk] Rule values, owner: product owner.** Channel limits shown (200 and 120 characters, banned word counts) are draft values pending Q-003.

## Build work named, not done

- Routes that wire each view to live data (TanStack Query over the generated API client).
- A Brands screen for voice notes.
- `apps/web/src/features/shell/format.ts` was written for the designs (INR and micro-USD formatting); review it as product code.
