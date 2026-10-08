import type { ScreenSpec } from "@/design/screen";
import { AppFrame } from "@/features/shell/components/AppFrame";
import { budgetBlocked, budgetOk, seller } from "@/features/shell/fixtures";

import { ProductFixDrawer } from "../components/ProductFixDrawer";
import {
  ProductsView,
  type ProductRow,
  type ProductsViewProps,
  type RunCounts,
} from "../components/ProductsView";

// Per-product AI cost in micro-USD: detection (4,200) plus one listing per
// channel (5,000 each), the Q-018 estimate; a regeneration adds 1,100.
const DETECT = 4_200;
const LISTING = 5_000;
const REGEN = 1_100;

const NAMES = ["Colour", "Pattern", "Sleeve", "Neckline", "Fit"];

/** done: a detected product; attributes are given in NAMES order. */
const done = (sku: string, priceMinor: number, read: string, extra = 0): ProductRow => ({
  id: sku.slice(3),
  sku,
  brand: "Indigo Loom",
  category: "kurta",
  priceMinor,
  imageCount: 2,
  detection: "done",
  attributes: read.split(", ").map((value, i) => ({ name: NAMES[i] ?? "", value })),
  aiCostMicroUsd: DETECT + 2 * LISTING + extra,
});

const ku101 = done(
  "KU-101",
  129_900,
  "white, block print, three-quarter sleeve, round neck, regular",
);
const ku102 = done(
  "KU-102",
  149_900,
  "navy, solid, three-quarter sleeve, round neck, regular",
  REGEN,
);
const ku104 = done("KU-104", 149_900, "black, solid, full sleeve, mandarin collar, straight");
const ku117 = done("KU-117", 169_900, "rust, checked, sleeveless, v-neck, a-line");
const ku210 = done("KU-210", 99_900, "olive, striped, short sleeve, round neck, regular");

const products: ProductRow[] = [ku101, ku102, ku104, ku117, ku210];

const inFlight: ProductRow[] = [
  ku101,
  ku102,
  { ...ku104, detection: "pending", attributes: undefined, aiCostMicroUsd: 0 },
  {
    ...ku117,
    detection: "failed",
    attributes: undefined,
    detectionError: "The model could not read the photo after 3 tries",
    aiCostMicroUsd: 3 * DETECT,
  },
  { ...ku210, detection: "stopped_budget", attributes: undefined, aiCostMicroUsd: 0 },
];

// The S-03 launch: 297 products, 2 without a photo, so 295 are detected and
// each gets one listing on each of 2 channels.
const DETECTED = 297 - 2;
const LISTINGS = DETECTED * 2;

const running: RunCounts = {
  detection: { pending: 39, done: DETECTED - 39 - 1, failed: 1, stopped_budget: 0 },
  listings: { queued: 120, generated: LISTINGS - 120 - 2, failed: 2, stopped_budget: 0 },
};

// The budget stopped 36 products before detection, so their 72 listings stopped
// too; 14 more listings stopped mid-generation. The failed product's 2 listings failed.
const stopped: RunCounts = {
  detection: { pending: 0, done: DETECTED - 36 - 1, failed: 1, stopped_budget: 36 },
  listings: {
    queued: 0,
    generated: LISTINGS - 2 - (36 * 2 + 14),
    failed: 2,
    stopped_budget: 36 * 2 + 14,
  },
};

const framed = (props: ProductsViewProps, budget = budgetOk) => (
  <AppFrame current="products" {...seller} budget={budget}>
    <ProductsView totalProducts={props.products?.length ? 297 : 0} {...props} />
  </AppFrame>
);

export const screen: ScreenSpec = {
  id: "S-04",
  name: "Products and generation",
  feature: "catalogue",
  job: "Show each product's detection, listings progress and AI cost, and start or resume generation.",
  states: {
    loading: () => framed({ status: "loading" }),
    empty: () => framed({ status: "ready", products: [] }),
    success: () => framed({ status: "ready", products }),
    generating: () => framed({ status: "ready", products: inFlight, run: running }),
    "voice-note-required": () =>
      framed({ status: "ready", products, brandsWithoutVoice: ["Indigo Loom"] }),
    "budget-blocked": () =>
      framed(
        { status: "ready", products: inFlight, run: stopped, budgetBlocked: true },
        budgetBlocked,
      ),
    error: () => framed({ status: "error", requestId: "req_01J9Z6" }),
    // A product without a photo opened from its Fix button.
    "fix-drawer": () => (
      <>
        {framed({ status: "ready", products })}
        <ProductFixDrawer
          target={{
            key: "product:210",
            title: "KU-210",
            problem: "No photo yet, so the AI cannot read this product.",
            values: { sku: "KU-210", category: "kurta", brand: "Indigo Loom", price: "1299" },
            photoCount: 0,
          }}
          categories={["kurta", "kurti", "saree", "lehenga", "salwar suit", "sherwani", "dupatta"]}
          onClose={() => undefined}
        />
      </>
    ),
  },
};
