import type { ScreenSpec } from "@/design/screen";
import { AppFrame, type Role } from "@/features/shell/components/AppFrame";
import { budgetOk, reviewer, seller } from "@/features/shell/fixtures";

import {
  ReviewGridView,
  type GridRow,
  type ReviewGridViewProps,
} from "../components/ReviewGridView";

// Per-product facts every channel row of that product shares. Cost in
// micro-USD: one enrichment call (6,100), plus one title rewrite (1,100) on KU-104.
const PRODUCT: Record<string, Partial<GridRow>> = {
  "KU-102": { costMicroUsd: 6_100, confidence: 0.93 },
  "KU-104": { costMicroUsd: 6_100 + 1_100, confidence: 0.91 },
  "KU-117": { costMicroUsd: 6_100, confidence: 0.48 },
};

const base = (
  id: string,
  sku: string,
  channel: string,
  title: string,
  attributes: string,
  extra: Partial<GridRow> = {},
): GridRow => ({
  id,
  sku,
  channel,
  version: 1,
  title,
  bullets: [
    "Soft, breathable cotton for all-day wear",
    "Regular fit that sits straight from the shoulder",
    "Three-quarter sleeves",
    "Round neckline with a placket",
    "Machine wash cold",
  ],
  description: "An everyday cotton kurta in a deep navy, cut straight with three-quarter sleeves.",
  attributes,
  ruleStatus: "passing",
  ruleFailures: [],
  approved: false,
  ...PRODUCT[sku],
  ...extra,
});

const navy = "navy, solid, three-quarter sleeve, round neck, regular";
// A title over the Amazon-style limit of 200 characters; the message counts it.
const longTitle =
  "Indigo Loom Women's Black Cotton Kurta, Solid, Straight Fit, Full Sleeve, Mandarin Collar, Everyday Ethnic Wear for Office, Festive and Casual Occasions, Machine Washable, Breathable Fabric, Sizes XS to XXL";

const rows: GridRow[] = [
  base(
    "412",
    "KU-102",
    "amazon_style",
    "Women's Navy Cotton Kurta, Regular Fit, Three-Quarter Sleeve",
    navy,
    {
      version: 3,
      approved: true,
      approvedAt: "2026-10-04T10:12:00Z",
      approvedBy: "asha.reviewer@example.in",
    },
  ),
  base("413", "KU-102", "own_website", "Navy Cotton Kurta", navy),
  base(
    "418",
    "KU-104",
    "amazon_style",
    longTitle,
    "black, solid, full sleeve, mandarin collar, straight",
    {
      ruleStatus: "failing",
      ruleFailures: [
        {
          rule: "title_max_length",
          field: "title",
          message: `Title is ${longTitle.length} characters; the Amazon-style limit is 200.`,
        },
      ],
    },
  ),
  base(
    "419",
    "KU-104",
    "own_website",
    "Black Cotton Kurta with Mandarin Collar",
    "black, solid, full sleeve, mandarin collar, straight",
    {
      ruleStatus: "failing",
      ruleFailures: [
        {
          rule: "banned_word",
          field: "bullet_5",
          message: 'Bullet 5 uses the banned phrase "easy care".',
        },
      ],
    },
  ),
  base(
    "431",
    "KU-117",
    "amazon_style",
    "Women's Rust Checked Cotton Kurta, A-Line, Sleeveless, V-Neck",
    "rust, checked, sleeveless, v-neck, a-line",
  ),
  base(
    "432",
    "KU-117",
    "own_website",
    "Rust Checked A-Line Kurta",
    "rust, checked, sleeveless, v-neck, a-line",
    {
      approved: true,
      approvedAt: "2026-10-04T10:14:00Z",
      approvedBy: "asha.reviewer@example.in",
    },
  ),
];

const framed = (
  props: Omit<ReviewGridViewProps, "role">,
  who: { role: Role; email: string } = reviewer,
) => (
  <AppFrame current="review" {...who} budget={budgetOk}>
    <ReviewGridView role={who.role} {...props} />
  </AppFrame>
);

export const screen: ScreenSpec = {
  id: "S-05",
  name: "Review listings",
  feature: "listings",
  job: "Let a reviewer see every listing's rule result, fix it, and approve only what passes.",
  states: {
    loading: () => framed({ status: "loading" }),
    empty: () => framed({ status: "ready", rows: [] }),
    success: () => framed({ status: "ready", rows, selectedIds: ["413", "431"] }),
    "filtered-empty": () =>
      framed({
        status: "ready",
        rows: rows.filter((r) => r.ruleStatus === "passing"),
        filter: "failing",
      }),
    editing: () => framed({ status: "ready", rows, editingId: "418" }),
    "version-conflict": () => framed({ status: "ready", rows, editingId: "413", conflict: true }),
    "approve-result": () =>
      framed({
        status: "ready",
        rows,
        approveResult: {
          approved: 1,
          skipped: [
            { sku: "KU-104", channel: "amazon_style", reason: "failing_rules" },
            { sku: "KU-102", channel: "own_website", reason: "version_changed" },
          ],
        },
      }),
    "regeneration-superseded": () =>
      framed({
        status: "ready",
        rows: rows.map((r) =>
          r.id === "431"
            ? { ...r, regeneration: { field: "title", status: "superseded" as const } }
            : r,
        ),
        editingId: "431",
      }),
    "rules-changed": () =>
      framed({ status: "ready", rows, clearedByRecheck: { channel: "amazon_style", count: 4 } }),
    "seller-read-only": () => framed({ status: "ready", rows, editingId: "412" }, seller),
    error: () => framed({ status: "error", requestId: "req_01J9Z6" }),
  },
};
