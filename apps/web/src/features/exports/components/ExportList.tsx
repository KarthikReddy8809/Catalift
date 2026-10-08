import { Download, Send } from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";

export interface ExportListItem {
  id: string;
  /** When the files were written, as shown. */
  createdAt: string;
  /** When a reviewer sent it to the seller, as shown; absent until sent. */
  sentAt?: string | undefined;
  sentBy?: string | undefined;
  files: { channel: string; name: string; rowCount: number; downloadUrl: string }[];
}

export interface ExportListProps {
  exports: ExportListItem[];
  /** Reviewers send an export to the seller; sellers get no send control. */
  onSend?: ((id: string) => void) | undefined;
  /** The export being sent, so only its button shows progress. */
  sendingId?: string | undefined;
}

/**
 * ExportList: GET /v1/exports. The reviewer sees every export and sends each
 * to the seller once; the seller sees what was sent and downloads it
 * (ADR-0012).
 */
export function ExportList({ exports, onSend, sendingId }: ExportListProps) {
  return (
    <ul className="space-y-3">
      {exports.map((e) => (
        <li
          key={e.id}
          className="flex flex-wrap items-start justify-between gap-4 rounded-xl border bg-card p-4 shadow-sm"
        >
          <div className="min-w-0 space-y-2">
            <div className="flex flex-wrap items-center gap-2">
              <h3 className="font-medium">Export {e.id}</h3>
              {e.sentAt ? (
                <Badge variant="secondary">Sent to seller</Badge>
              ) : onSend ? (
                <Badge variant="outline">Not sent</Badge>
              ) : null}
            </div>
            <p className="text-sm text-muted-foreground">
              Written {e.createdAt}
              {e.sentAt ? ` · sent ${e.sentAt}${e.sentBy ? ` by ${e.sentBy}` : ""}` : ""}
            </p>
            <ul className="flex flex-wrap gap-2">
              {e.files.map((f) => (
                <li key={f.channel}>
                  <Button asChild variant="outline" size="sm" className="h-9">
                    <a href={f.downloadUrl} download>
                      <Download aria-hidden />
                      {f.name} CSV ({f.rowCount} rows)
                    </a>
                  </Button>
                </li>
              ))}
            </ul>
          </div>
          {onSend && !e.sentAt ? (
            <Button
              className="h-11"
              disabled={sendingId === e.id}
              onClick={() => {
                onSend(e.id);
              }}
              aria-label={`Send export ${e.id} to the seller`}
            >
              <Send aria-hidden />
              {sendingId === e.id ? "Sending…" : "Send to seller"}
            </Button>
          ) : null}
        </li>
      ))}
    </ul>
  );
}
