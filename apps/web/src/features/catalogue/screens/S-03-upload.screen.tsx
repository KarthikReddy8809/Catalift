import type { ScreenSpec } from "@/design/screen";
import { AppFrame } from "@/features/shell/components/AppFrame";
import { budgetOk, seller } from "@/features/shell/fixtures";

import { UploadView, type UploadSummary, type UploadViewProps } from "../components/UploadView";

const summary: UploadSummary = {
  fileName: "indigo-loom-autumn.csv",
  rowsTotal: 300,
  rowsAccepted: 297,
  rowsRejected: 3,
  rowErrors: [
    { rowNumber: 17, sku: "KU-104", reason: "SKU already exists" },
    { rowNumber: 40, sku: null, reason: "sku is missing" },
    { rowNumber: 212, sku: "KU-311", reason: "price must be a positive number" },
  ],
};

const missingImage = ["KU-210", "KU-288"];

const framed = (props: UploadViewProps) => (
  <AppFrame current="upload" {...seller} budget={budgetOk}>
    <UploadView {...props} />
  </AppFrame>
);

export const screen: ScreenSpec = {
  id: "S-03",
  name: "Upload a launch",
  feature: "catalogue",
  job: "Get a launch's products and photos into Catalift, and say exactly which rows and files did not make it.",
  states: {
    empty: () => framed({ step: "csv" }),
    loading: () =>
      framed({ step: "uploading", uploadingFile: "indigo-loom-autumn.csv", progress: 60 }),
    success: () => framed({ step: "csv", summary }),
    "images-attached": () =>
      framed({
        step: "images",
        summary,
        images: {
          // Two photos per product, except the two products still without one.
          attached: (summary.rowsAccepted - missingImage.length) * 2,
          unmatchedFiles: ["holiday-banner.png", "TS10-front.jpg"],
          productsMissingImage: missingImage,
        },
      }),
    "file-too-large": () =>
      framed({ step: "csv", problem: { kind: "too-large", fileName: "full-catalogue.csv" } }),
    "wrong-type": () =>
      framed({ step: "csv", problem: { kind: "wrong-type", fileName: "catalogue.xlsx" } }),
    conflict: () => framed({ step: "csv", problem: { kind: "conflict" } }),
    error: () => framed({ step: "csv", problem: { kind: "server", requestId: "req_01J9Z6" } }),
  },
};
