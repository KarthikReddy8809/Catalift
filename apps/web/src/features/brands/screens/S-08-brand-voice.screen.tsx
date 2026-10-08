import type { ScreenSpec } from "@/design/screen";
import { AppFrame } from "@/features/shell/components/AppFrame";
import { budgetOk, seller } from "@/features/shell/fixtures";

import {
  BrandVoiceView,
  type BrandRow,
  type BrandVoiceViewProps,
} from "../components/BrandVoiceView";

const brands: BrandRow[] = [
  {
    id: "3",
    name: "Indigo Loom",
    tone: "Warm and plain-spoken; mention the fabric first.",
    wordsToAvoid: ["cheap", "best ever"],
    version: "1",
  },
  { id: "4", name: "Monsoon Bazaar", tone: "", wordsToAvoid: [], version: "1" },
];

const framed = (props: BrandVoiceViewProps) => (
  <AppFrame current="brands" {...seller} budget={budgetOk}>
    <BrandVoiceView {...props} />
  </AppFrame>
);

export const screen: ScreenSpec = {
  id: "S-08",
  name: "Brand voice",
  feature: "brands",
  job: "Let the seller set each brand's tone and the words it never uses.",
  states: {
    loading: () => framed({ status: "loading" }),
    empty: () => framed({ status: "ready", brands: [] }),
    success: () => framed({ status: "ready", brands }),
    saving: () => framed({ status: "ready", brands, savingId: "3" }),
    saved: () =>
      framed({
        status: "ready",
        brands,
        outcome: {
          brandId: "3",
          tone: "success",
          message: "Indigo Loom's listings were checked for 2 words to avoid.",
        },
      }),
    "save-failed": () =>
      framed({
        status: "ready",
        brands,
        outcome: {
          brandId: "3",
          tone: "error",
          message:
            "words_to_avoid: each word or phrase must be at most 60 characters (request req_01J9Z7)",
        },
      }),
    error: () => framed({ status: "error", requestId: "req_01J9Z6" }),
  },
};
