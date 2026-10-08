import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Skeleton } from "@/components/ui/skeleton";

import { ExportList, type ExportListItem } from "./ExportList";

export interface ReceivedViewProps {
  status: "loading" | "ready" | "error";
  exports?: ExportListItem[];
  requestId?: string | undefined;
}

/**
 * ReceivedView: GET /v1/exports as a seller, which returns only the exports a
 * reviewer sent. Each holds one CSV per channel of approved listings, ready
 * to upload to that channel (ADR-0012).
 */
export function ReceivedView({ status, exports = [], requestId }: ReceivedViewProps) {
  return (
    <div className="space-y-6">
      <header className="space-y-1">
        <h1 className="text-2xl font-semibold tracking-tight">Received files</h1>
        <p className="max-w-prose text-muted-foreground">
          Approved listings a reviewer sent you, one CSV per channel, newest first. Each file is
          ready to upload to its channel.
        </p>
      </header>
      {status === "loading" ? (
        <div className="space-y-3" aria-busy="true" aria-label="Loading received files">
          <Skeleton className="h-28 w-full" />
          <Skeleton className="h-28 w-full" />
        </div>
      ) : status === "error" ? (
        <Alert variant="destructive" role="alert">
          <AlertTitle>Received files could not be loaded</AlertTitle>
          <AlertDescription>
            Reload the page. If it keeps failing, quote {requestId ?? "the time it happened"} to the
            Catalift team.
          </AlertDescription>
        </Alert>
      ) : exports.length === 0 ? (
        <div className="space-y-2 rounded-xl border bg-card p-8 shadow-sm">
          <h2 className="font-medium">Nothing sent yet</h2>
          <p className="max-w-prose text-muted-foreground">
            A reviewer approves your listings, exports them and sends the files here. Check Products
            to see where each product is.
          </p>
        </div>
      ) : (
        <ExportList exports={exports} />
      )}
    </div>
  );
}
