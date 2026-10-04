import { useState } from "react";

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Label } from "@/components/ui/label";
import { Progress } from "@/components/ui/progress";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableFooter,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { formatInrMinor, formatUsdMicro } from "@/features/shell/format";

export type DetectionStatus = "pending" | "done" | "failed" | "stopped_budget";

export interface ProductRow {
  sku: string;
  brand: string;
  category: string;
  priceMinor: number;
  imageCount: number;
  detection: DetectionStatus;
  detectionError?: string | undefined;
  /** Short attribute line, e.g. "navy, solid, round neck". */
  attributes?: string | undefined;
  aiCostMicroUsd: number;
}

export interface RunCounts {
  detection: Record<DetectionStatus, number>;
  listings: { queued: number; generated: number; failed: number; stopped_budget: number };
}

export interface ProductsViewProps {
  status: "loading" | "ready" | "error";
  products?: ProductRow[];
  /** Products in the upload; the list below may show one page of them. */
  totalProducts?: number;
  run?: RunCounts;
  /** Brands in scope with no voice note (AC-US-00-003-5); known up front from GET /v1/brands. */
  brandsWithoutVoice?: string[];
  budgetBlocked?: boolean;
  requestId?: string;
  /** The upload's file name, shown under the heading. */
  uploadName?: string | undefined;
  /** Starts a run; neutral is the seller's confirmation for brands without a note. */
  onGenerate?: (neutral: boolean) => void;
  /** Re-queues the run's failed and stopped work (D6). */
  onResume?: () => void;
  /** Goes to the upload screen. */
  onUploadLaunch?: () => void;
}

const DETECTION_LABEL: Record<DetectionStatus, string> = {
  pending: "Detecting",
  done: "Detected",
  failed: "Failed",
  stopped_budget: "Stopped: budget",
};

function DetectionBadge({ status }: { status: DetectionStatus }) {
  const variant =
    status === "done" ? "secondary" : status === "pending" ? "outline" : "destructive";
  return <Badge variant={variant}>{DETECTION_LABEL[status]}</Badge>;
}

function RunPanel({
  run,
  budgetBlocked,
  onResume,
}: {
  run: RunCounts;
  budgetBlocked: boolean;
  onResume?: (() => void) | undefined;
}) {
  const d = run.detection;
  const dTotal = d.pending + d.done + d.failed + d.stopped_budget;
  const l = run.listings;
  const lTotal = l.queued + l.generated + l.failed + l.stopped_budget;
  const stuck = d.failed + d.stopped_budget + l.failed + l.stopped_budget;
  const running = d.pending + l.queued > 0;
  return (
    <section
      aria-labelledby="run"
      className="grid gap-4 rounded-lg border p-4 md:grid-cols-[1fr_1fr_auto] md:items-end"
    >
      <h2 id="run" className="sr-only">
        Generation progress
      </h2>
      <div className="space-y-2">
        <p className="text-sm">
          Detection <span className="tabular-nums">{d.done}</span> of{" "}
          <span className="tabular-nums">{dTotal}</span> products
        </p>
        <Progress value={dTotal ? (d.done / dTotal) * 100 : 0} aria-label="Detection progress" />
      </div>
      <div className="space-y-2">
        <p className="text-sm">
          Listings <span className="tabular-nums">{l.generated}</span> of{" "}
          <span className="tabular-nums">{lTotal}</span> written
        </p>
        <Progress value={lTotal ? (l.generated / lTotal) * 100 : 0} aria-label="Listing progress" />
      </div>
      <div className="space-y-2 md:text-right">
        {running ? (
          <p className="text-sm text-muted-foreground" role="status">
            Running; you can close this page and come back.
          </p>
        ) : null}
        {stuck > 0 ? (
          <Button variant="outline" className="h-11" disabled={budgetBlocked} onClick={onResume}>
            Resume {stuck} failed and stopped
          </Button>
        ) : null}
      </div>
    </section>
  );
}

/**
 * ProductsView: GET /v1/products and GET /v1/generation-runs/{id}; starts runs
 * with POST /v1/generation-runs and resumes with .../resume (D6). Cost per
 * product and the batch total are AC-US-00-011-3 and -4.
 */
export function ProductsView({
  status,
  products = [],
  totalProducts,
  run,
  brandsWithoutVoice = [],
  budgetBlocked = false,
  requestId,
  uploadName = "indigo-loom-autumn.csv",
  onGenerate,
  onResume,
  onUploadLaunch,
}: ProductsViewProps) {
  const [neutral, setNeutral] = useState(false);
  const total = products.reduce((s, p) => s + p.aiCostMicroUsd, 0);
  const missingPhoto = products.filter((p) => p.imageCount === 0).length;
  return (
    <div className="space-y-6">
      <header className="flex flex-wrap items-end justify-between gap-4">
        <div className="space-y-1">
          <h1 className="text-2xl font-semibold tracking-tight">Products</h1>
          <p className="text-muted-foreground">
            {uploadName} &middot; {totalProducts ?? products.length} products
            {missingPhoto ? ` · ${missingPhoto} without a photo` : ""}
          </p>
        </div>
        {status === "ready" && products.length > 0 && !run ? (
          <Button
            className="h-11"
            disabled={budgetBlocked || brandsWithoutVoice.length > 0}
            onClick={() => onGenerate?.(false)}
          >
            Generate listings
          </Button>
        ) : null}
      </header>

      {brandsWithoutVoice.length > 0 ? (
        <Alert role="alert">
          <AlertTitle>
            {brandsWithoutVoice.join(", ")} {brandsWithoutVoice.length === 1 ? "has" : "have"} no
            voice note
          </AlertTitle>
          <AlertDescription className="space-y-3">
            <p>
              Listings follow each brand&rsquo;s voice note. Add one in Brands, or confirm that a
              plain, neutral voice is fine for this run.
            </p>
            <div className="flex items-center gap-3">
              <Checkbox
                id="neutral"
                className="size-5"
                checked={neutral}
                onCheckedChange={(v) => {
                  setNeutral(v === true);
                }}
              />
              <Label htmlFor="neutral">Use a neutral voice for brands without a note</Label>
            </div>
            <Button
              className="h-11"
              disabled={!neutral || budgetBlocked}
              onClick={() => onGenerate?.(true)}
            >
              Generate listings
            </Button>
          </AlertDescription>
        </Alert>
      ) : null}

      {status === "error" ? (
        <Alert variant="destructive" role="alert">
          <AlertTitle>Products could not be loaded</AlertTitle>
          <AlertDescription>
            Reload the page. If it keeps failing, quote {requestId} to the Catalift team.
          </AlertDescription>
        </Alert>
      ) : null}

      {run ? <RunPanel run={run} budgetBlocked={budgetBlocked} onResume={onResume} /> : null}

      {status === "loading" ? (
        <div className="space-y-2" aria-busy="true" aria-label="Loading products">
          {Array.from({ length: 6 }, (_, i) => (
            <Skeleton key={i} className="h-12 w-full" />
          ))}
        </div>
      ) : status === "ready" && products.length === 0 ? (
        <div className="space-y-3 rounded-lg border p-8">
          <h2 className="font-medium">No products yet</h2>
          <p className="max-w-prose text-muted-foreground">
            Upload the launch&rsquo;s product list and photos; products appear here as soon as the
            CSV is read.
          </p>
          <Button className="h-11" onClick={onUploadLaunch}>
            Upload a launch
          </Button>
        </div>
      ) : status === "ready" ? (
        <>
          <div className="hidden md:block">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>SKU</TableHead>
                  <TableHead>Brand</TableHead>
                  <TableHead className="text-right">Price</TableHead>
                  <TableHead>Photos</TableHead>
                  <TableHead>Detection</TableHead>
                  <TableHead>Attributes</TableHead>
                  <TableHead className="text-right">AI cost</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {products.map((p) => (
                  <TableRow key={p.sku}>
                    <TableCell className="font-medium">{p.sku}</TableCell>
                    <TableCell>{p.brand}</TableCell>
                    <TableCell className="text-right tabular-nums">
                      {formatInrMinor(p.priceMinor)}
                    </TableCell>
                    <TableCell className="tabular-nums">
                      {p.imageCount === 0 ? (
                        <span className="text-destructive">None</span>
                      ) : (
                        p.imageCount
                      )}
                    </TableCell>
                    <TableCell>
                      <DetectionBadge status={p.detection} />
                    </TableCell>
                    <TableCell className="max-w-64 truncate text-muted-foreground">
                      {p.detectionError ?? p.attributes ?? ""}
                    </TableCell>
                    <TableCell className="text-right tabular-nums">
                      {formatUsdMicro(p.aiCostMicroUsd)}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
              <TableFooter>
                <TableRow>
                  <TableCell colSpan={6}>Total for this upload</TableCell>
                  <TableCell className="text-right tabular-nums">{formatUsdMicro(total)}</TableCell>
                </TableRow>
              </TableFooter>
            </Table>
          </div>

          <ul className="space-y-2 md:hidden" aria-label="Products">
            {products.map((p) => (
              <li key={p.sku} className="space-y-1 rounded-lg border p-3">
                <div className="flex items-center justify-between gap-2">
                  <span className="font-medium">{p.sku}</span>
                  <DetectionBadge status={p.detection} />
                </div>
                <p className="truncate text-sm text-muted-foreground">
                  {p.detectionError ?? p.attributes ?? `${p.imageCount} photos`}
                </p>
                <p className="flex justify-between text-sm tabular-nums">
                  <span>{formatInrMinor(p.priceMinor)}</span>
                  <span>AI {formatUsdMicro(p.aiCostMicroUsd)}</span>
                </p>
              </li>
            ))}
            <li className="flex justify-between px-3 text-sm font-medium tabular-nums">
              <span>Total for this upload</span>
              <span>{formatUsdMicro(total)}</span>
            </li>
          </ul>
        </>
      ) : null}
    </div>
  );
}
