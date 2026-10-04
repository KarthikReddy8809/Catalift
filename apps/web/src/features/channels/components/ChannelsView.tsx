import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

export interface ChannelRow {
  id: string;
  name: string;
  enabled: boolean;
  loadError: string | null;
  titleMaxLength: number | null;
  requiredAttributes: string[];
  bannedWordCount: number;
  lastRecheck: { listingsRechecked: number; approvalsCleared: number; when: string } | null;
}

export interface ChannelsViewProps {
  status: "loading" | "ready" | "error";
  channels?: ChannelRow[];
  requestId?: string;
}

/**
 * ChannelsView: GET /v1/channels. Read-only: rules are configuration files
 * changed by pull request (REQ-017), so there is no edit control here.
 */
export function ChannelsView({ status, channels = [], requestId }: ChannelsViewProps) {
  const broken = channels.filter((c) => c.loadError);
  return (
    <div className="space-y-6">
      <header className="space-y-1">
        <h1 className="text-2xl font-semibold tracking-tight">Channels</h1>
        <p className="max-w-prose text-muted-foreground">
          The rules every listing is checked against. They change through a pull request to the
          channel files; after a change, existing listings are checked again.
        </p>
      </header>

      {broken.map((c) => (
        <Alert key={c.id} variant="destructive" role="alert">
          <AlertTitle>{c.name} is switched off</AlertTitle>
          <AlertDescription>
            Its rules file could not be read: {c.loadError}. Listings for {c.name} are not written,
            checked or exported until the file is fixed and deployed. Other channels work as normal.
          </AlertDescription>
        </Alert>
      ))}

      {status === "error" ? (
        <Alert variant="destructive" role="alert">
          <AlertTitle>Channels could not be loaded</AlertTitle>
          <AlertDescription>
            Reload the page. If it keeps failing, quote {requestId} to the Catalift team.
          </AlertDescription>
        </Alert>
      ) : status === "loading" ? (
        <div className="space-y-2" aria-busy="true" aria-label="Loading channels">
          <Skeleton className="h-12 w-full" />
          <Skeleton className="h-12 w-full" />
        </div>
      ) : (
        <div className="overflow-x-auto">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Channel</TableHead>
                <TableHead>State</TableHead>
                <TableHead className="text-right">Title limit</TableHead>
                <TableHead>Required attributes</TableHead>
                <TableHead className="text-right">Banned words</TableHead>
                <TableHead>Last re-check</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {channels.map((c) => (
                <TableRow key={c.id}>
                  <TableCell className="font-medium">{c.name}</TableCell>
                  <TableCell>
                    {c.enabled ? (
                      <Badge variant="secondary">On</Badge>
                    ) : (
                      <Badge variant="destructive">Off: file error</Badge>
                    )}
                  </TableCell>
                  <TableCell className="text-right tabular-nums">
                    {c.titleMaxLength ?? <span className="text-muted-foreground">none</span>}
                  </TableCell>
                  <TableCell>{c.requiredAttributes.join(", ") || "none"}</TableCell>
                  <TableCell className="text-right tabular-nums">{c.bannedWordCount}</TableCell>
                  <TableCell className="text-sm">
                    {c.lastRecheck ? (
                      <>
                        {c.lastRecheck.listingsRechecked} checked, {c.lastRecheck.approvalsCleared}{" "}
                        approvals cleared
                        <span className="block text-muted-foreground">{c.lastRecheck.when}</span>
                      </>
                    ) : (
                      <span className="text-muted-foreground">Never changed</span>
                    )}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}
    </div>
  );
}
