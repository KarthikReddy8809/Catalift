import type { ScreenSpec } from "@/design/screen";
import { AppFrame } from "@/features/shell/components/AppFrame";
import { budgetOk, seller } from "@/features/shell/fixtures";

import type { ExportListItem } from "../components/ExportList";
import { ReceivedView, type ReceivedViewProps } from "../components/ReceivedView";

const received: ExportListItem[] = [
  {
    id: "12",
    createdAt: "7 Oct 2026, 3:10 pm IST",
    sentAt: "7 Oct 2026, 3:12 pm IST",
    sentBy: "asha.reviewer@example.in",
    files: [
      { channel: "amazon_style", name: "Amazon-style", rowCount: 289, downloadUrl: "#" },
      { channel: "own_website", name: "Own website", rowCount: 289, downloadUrl: "#" },
    ],
  },
  {
    id: "9",
    createdAt: "4 Oct 2026, 4:30 pm IST",
    sentAt: "4 Oct 2026, 4:35 pm IST",
    sentBy: "asha.reviewer@example.in",
    files: [{ channel: "amazon_style", name: "Amazon-style", rowCount: 150, downloadUrl: "#" }],
  },
];

const framed = (props: ReceivedViewProps) => (
  <AppFrame current="received" {...seller} budget={budgetOk}>
    <ReceivedView {...props} />
  </AppFrame>
);

export const screen: ScreenSpec = {
  id: "S-09",
  name: "Received files",
  feature: "exports",
  job: "Give the seller the approved CSVs a reviewer sent, one per channel, ready to upload.",
  states: {
    loading: () => framed({ status: "loading" }),
    empty: () => framed({ status: "ready", exports: [] }),
    success: () => framed({ status: "ready", exports: received }),
    error: () => framed({ status: "error", requestId: "req_01J9Z6" }),
  },
};
