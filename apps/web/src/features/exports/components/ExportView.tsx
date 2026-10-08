import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

import { ExportList, type ExportListItem } from "./ExportList";

export interface ChannelReadiness {
  channel: string;
  name: string;
  approved: number;
  total: number;
}

export interface ExportFile {
  channel: string;
  name: string;
  rowCount: number;
  downloadUrl: string;
}

export interface ExportViewProps {
  status: "ready" | "exporting" | "done" | "nothing-approved" | "error";
  channels: ChannelReadiness[];
  files?: ExportFile[];
  skipped?: { name: string; reason: string }[];
  requestId?: string;
  /** Writes the files; absent in the design gallery. */
  onExport?: () => void;
  /** Earlier exports, newest first, each sent to the seller once (ADR-0012). */
  history?: ExportListItem[];
  onSend?: ((id: string) => void) | undefined;
  sendingId?: string | undefined;
  /** Why the last send failed, shown above the list. */
  sendError?: string | undefined;
}

/**
 * ExportView: POST /v1/exports, then GET /v1/exports/{id}/files/{channel}.
 * Only listings approved at their current version are written (REQ-015);
 * a channel with none gets no file (AC-US-00-010-5).
 */
export function ExportView({
  status,
  channels,
  files = [],
  skipped = [],
  requestId,
  onExport,
  history = [],
  onSend,
  sendingId,
  sendError,
}: ExportViewProps) {
  const approvedTotal = channels.reduce((s, c) => s + c.approved, 0);
  return (
    <div className="space-y-6">
      <header className="flex flex-wrap items-end justify-between gap-4">
        <div className="space-y-1">
          <h1 className="text-2xl font-semibold tracking-tight">Export</h1>
          <p className="max-w-prose text-muted-foreground">
            One CSV per channel, with only approved listings, in the column order each
            channel&rsquo;s upload template expects.
          </p>
        </div>
        <Button
          className="h-11"
          disabled={approvedTotal === 0 || status === "exporting"}
          onClick={onExport}
        >
          {status === "exporting" ? "Writing files…" : `Export ${approvedTotal} approved listings`}
        </Button>
      </header>

      {status === "done" ? (
        <Alert role="status">
          <AlertTitle>Files written</AlertTitle>
          <AlertDescription>
            Check them, then use Send to seller below so the seller can download them.
          </AlertDescription>
        </Alert>
      ) : null}
      {status === "nothing-approved" ? (
        <Alert role="alert">
          <AlertTitle>No listing is approved yet</AlertTitle>
          <AlertDescription>
            Nothing would be written. Approve listings in Review, then export.
          </AlertDescription>
        </Alert>
      ) : null}
      {status === "error" ? (
        <Alert variant="destructive" role="alert">
          <AlertTitle>The export did not finish</AlertTitle>
          <AlertDescription>
            No file was written. Try again; if it fails again, quote {requestId} to the Catalift
            team.
          </AlertDescription>
        </Alert>
      ) : null}

      <div className="overflow-hidden rounded-xl border bg-card shadow-sm">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Channel</TableHead>
              <TableHead className="text-right">Approved</TableHead>
              <TableHead className="text-right">Not yet</TableHead>
              <TableHead>File</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {channels.map((c) => {
              const file = files.find((f) => f.channel === c.channel);
              const skip = skipped.find((s) => s.name === c.name);
              return (
                <TableRow key={c.channel}>
                  <TableCell className="font-medium">{c.name}</TableCell>
                  <TableCell className="text-right tabular-nums">{c.approved}</TableCell>
                  <TableCell className="text-right tabular-nums">{c.total - c.approved}</TableCell>
                  <TableCell>
                    {file ? (
                      <Button asChild variant="link" className="h-11 px-0">
                        <a href={file.downloadUrl} download>
                          Download {file.name} CSV ({file.rowCount} rows)
                        </a>
                      </Button>
                    ) : skip ? (
                      <span className="text-muted-foreground">No file: {skip.reason}</span>
                    ) : (
                      <span className="text-muted-foreground">Not exported yet</span>
                    )}
                  </TableCell>
                </TableRow>
              );
            })}
          </TableBody>
        </Table>
      </div>
      <p className="text-sm text-muted-foreground">
        Listings edited after approval are left out until they are approved again. Cells that start
        with =, +, - or @ are escaped so spreadsheets do not run them.
      </p>

      <section aria-labelledby="exports-sent" className="space-y-3">
        <div className="space-y-1">
          <h2 id="exports-sent" className="text-lg font-semibold tracking-tight">
            Send to the seller
          </h2>
          <p className="text-sm text-muted-foreground">
            The seller sees an export, and can download its files, only after you send it.
          </p>
        </div>
        {sendError ? (
          <Alert variant="destructive" role="alert">
            <AlertTitle>The export was not sent</AlertTitle>
            <AlertDescription>{sendError}</AlertDescription>
          </Alert>
        ) : null}
        {history.length > 0 ? (
          <ExportList exports={history} onSend={onSend} sendingId={sendingId} />
        ) : (
          <p className="rounded-xl border border-dashed p-6 text-sm text-muted-foreground">
            No export yet. Export the approved listings above, then send them here.
          </p>
        )}
      </section>
    </div>
  );
}
