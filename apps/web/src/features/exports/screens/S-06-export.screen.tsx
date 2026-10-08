import type { ScreenSpec } from "@/design/screen";
import { AppFrame } from "@/features/shell/components/AppFrame";
import { budgetOk, reviewer } from "@/features/shell/fixtures";

import type { ExportListItem } from "../components/ExportList";
import { ExportView, type ChannelReadiness, type ExportViewProps } from "../components/ExportView";

// The launch from S-03: 297 products, one listing per channel each.
const PRODUCTS = 297;
const AMAZON_APPROVED = 289;
const channels: ChannelReadiness[] = [
  { channel: "amazon_style", name: "Amazon-style", approved: AMAZON_APPROVED, total: PRODUCTS },
  { channel: "own_website", name: "Own website", approved: 0, total: PRODUCTS },
];

const none: ChannelReadiness[] = channels.map((c) => ({ ...c, approved: 0 }));

const files = [
  { channel: "amazon_style", name: "Amazon-style", rowCount: AMAZON_APPROVED, downloadUrl: "#" },
];
const skipped = [{ name: "Own website", reason: "no approved listings" }];

const history: ExportListItem[] = [
  { id: "12", createdAt: "7 Oct 2026, 3:10 pm IST", files },
  {
    id: "9",
    createdAt: "4 Oct 2026, 4:30 pm IST",
    sentAt: "4 Oct 2026, 4:35 pm IST",
    sentBy: "asha.reviewer@example.in",
    files: [
      { channel: "amazon_style", name: "Amazon-style", rowCount: 150, downloadUrl: "#" },
      { channel: "own_website", name: "Own website", rowCount: 150, downloadUrl: "#" },
    ],
  },
];

// The send control shows only with a handler; the gallery's does nothing.
const framed = (props: ExportViewProps) => (
  <AppFrame current="export" {...reviewer} budget={budgetOk}>
    <ExportView onSend={() => undefined} {...props} />
  </AppFrame>
);

export const screen: ScreenSpec = {
  id: "S-06",
  name: "Export",
  feature: "exports",
  job: "Give the reviewer one ready-to-upload CSV per channel, holding only approved listings, and send it to the seller.",
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
    // After an export: the newest is not sent yet, an older one was.
    "send-to-seller": () => framed({ status: "done", channels, files, skipped, history }),
    sending: () => framed({ status: "ready", channels, history, sendingId: "12" }),
    "send-failed": () =>
      framed({
        status: "ready",
        channels,
        history,
        sendError: "The export could not be found. (request req_01J9Z7)",
      }),
  },
};
