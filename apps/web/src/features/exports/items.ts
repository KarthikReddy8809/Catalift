import { formatIst } from "@/features/shell/format";

import { downloadHref, type Export } from "./api";
import type { ExportListItem } from "./components/ExportList";

/** toListItem shapes an export for ExportList; nameOf turns a channel id into its name. */
export function toListItem(e: Export, nameOf: (channel: string) => string): ExportListItem {
  return {
    id: e.id,
    createdAt: formatIst(e.created_at),
    sentAt: e.sent_at ? formatIst(e.sent_at) : undefined,
    sentBy: e.sent_by ?? undefined,
    files: e.files.map((f) => ({
      channel: f.channel,
      name: nameOf(f.channel),
      rowCount: f.row_count,
      downloadUrl: downloadHref(f.download_url),
    })),
  };
}
