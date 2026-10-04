import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import type { ScreenSpec } from "@/design/screen";
import { AppFrame } from "@/features/shell/components/AppFrame";
import { budgetOk, reviewer, seller } from "@/features/shell/fixtures";

import { ExportView, type ChannelReadiness, type ExportViewProps } from "../components/ExportView";

// The launch from S-03: 297 products, one listing per channel each.
const PRODUCTS = 297;
const AMAZON_APPROVED = 289;
const channels: ChannelReadiness[] = [
  { channel: "amazon_style", name: "Amazon-style", approved: AMAZON_APPROVED, total: PRODUCTS },
  { channel: "own_website", name: "Own website", approved: 0, total: PRODUCTS },
];

const none: ChannelReadiness[] = channels.map((c) => ({ ...c, approved: 0 }));

const framed = (props: ExportViewProps) => (
  <AppFrame current="export" {...reviewer} budget={budgetOk}>
    <ExportView {...props} />
  </AppFrame>
);

export const screen: ScreenSpec = {
  id: "S-06",
  name: "Export",
  feature: "exports",
  job: "Give the reviewer one ready-to-upload CSV per channel, holding only approved listings.",
  states: {
    ready: () => framed({ status: "ready", channels }),
    loading: () => framed({ status: "exporting", channels }),
    success: () =>
      framed({
        status: "done",
        channels,
        files: [
          {
            channel: "amazon_style",
            name: "Amazon-style",
            rowCount: AMAZON_APPROVED,
            downloadUrl: "#",
          },
        ],
        skipped: [{ name: "Own website", reason: "no approved listings" }],
      }),
    "nothing-approved": () => framed({ status: "nothing-approved", channels: none }),
    error: () => framed({ status: "error", channels, requestId: "req_01J9Z6" }),
    // A seller has no Export entry; a direct visit gets 403 forbidden_role.
    "seller-forbidden": () => (
      <AppFrame current="products" {...seller} budget={budgetOk}>
        <h1 className="text-2xl font-semibold tracking-tight">Export</h1>
        <Alert role="alert">
          <AlertTitle>Export is for reviewers</AlertTitle>
          <AlertDescription>
            Listings leave Catalift only after a reviewer approves them. Ask a reviewer to export,
            or carry on in Products.
          </AlertDescription>
        </Alert>
      </AppFrame>
    ),
  },
};
