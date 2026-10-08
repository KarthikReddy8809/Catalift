import { ChevronLeft, ChevronRight, Search, Trash2, Wrench } from "lucide-react";
import { useState } from "react";

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
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
  /** The product's id; needed to add a photo to it. */
  id?: string | undefined;
  /** The upload that created it; the table lists the newest upload first. */
  uploadId?: string | undefined;
  sku: string;
  brand: string;
  category: string;
  priceMinor: number;
  imageCount: number;
  detection: DetectionStatus;
  /** Where the product is in the flow (seller flow step 5); derived from detection when absent. */
  status?: ProductStatus | undefined;
  detectionError?: string | undefined;
  /** What the AI read from the photo, e.g. Colour navy, Pattern solid. */
  attributes?: ProductAttribute[] | undefined;
  aiCostMicroUsd: number;
}

export interface ProductAttribute {
  name: string;
  value: string;
}

/** Rows per page of the products table. */
export const PAGE_SIZE = 10;

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
  /** Why the list did not load, shown with the error. */
  errorMessage?: string | undefined;
  /** The upload's file name, shown under the heading. */
  uploadName?: string | undefined;
  /** Starts a run; neutral is the seller's confirmation for brands without a note. */
  onGenerate?: (neutral: boolean) => void;
  /** Re-queues the run's failed and stopped work (D6). */
  onResume?: () => void;
  /** Goes to the upload screen. */
  onUploadLaunch?: () => void;
  /** Sellers open a product that failed validation in the fix drawer. */
  onOpenFix?: ((row: ProductRow) => void) | undefined;
  /** Rows the upload refused, with the seller's way to fix them. */
  rejected?: RejectedRow[] | undefined;
  /** Sellers open a rejected row in the fix drawer. */
  onOpenRejected?: ((row: RejectedRow) => void) | undefined;
  /** Sellers drop every rejected row, such as duplicates of loaded products. */
  onDiscardAll?: (() => void) | undefined;
  discarding?: boolean | undefined;
}

export type ProductStatus =
  | "needs_photo"
  | "uploaded"
  | "enriching"
  | "ready_for_review"
  | "approved"
  | "failed"
  | "budget_exhausted";

const STATUS: Record<
  ProductStatus,
  { label: string; tone: "default" | "secondary" | "outline" | "destructive" }
> = {
  needs_photo: { label: "Needs photo", tone: "destructive" },
  uploaded: { label: "Uploaded", tone: "outline" },
  enriching: { label: "Enriching", tone: "outline" },
  ready_for_review: { label: "Ready for review", tone: "secondary" },
  approved: { label: "Approved", tone: "default" },
  failed: { label: "Failed", tone: "destructive" },
  budget_exhausted: { label: "Budget exhausted", tone: "destructive" },
};

const FROM_DETECTION: Record<DetectionStatus, ProductStatus> = {
  pending: "enriching",
  done: "ready_for_review",
  failed: "failed",
  stopped_budget: "budget_exhausted",
};

/** needsFix: the products a seller opens in the fix drawer. */
function needsFix(row: ProductRow): boolean {
  const status = row.status ?? FROM_DETECTION[row.detection];
  return (
    row.imageCount === 0 ||
    status === "needs_photo" ||
    status === "failed" ||
    status === "budget_exhausted"
  );
}

/** A CSV row the upload refused, as typed, with the reason. */
export interface RejectedRow {
  uploadId: string;
  fileName: string;
  rowNumber: number;
  sku: string;
  category: string;
  brand: string;
  price: string;
  reason: string;
}

const rejectedKey = (r: RejectedRow) => `rejected-${r.uploadId}-${String(r.rowNumber)}`;

function RejectedFix({
  row,
  onOpen,
}: {
  row: RejectedRow;
  onOpen?: ((row: RejectedRow) => void) | undefined;
}) {
  if (!onOpen) return null;
  return (
    <Button
      variant="outline"
      size="sm"
      className="h-8"
      onClick={(e) => {
        e.stopPropagation();
        onOpen(row);
      }}
      aria-label={`Fix row ${String(row.rowNumber)} of ${row.fileName}`}
    >
      <Wrench aria-hidden />
      Fix
    </Button>
  );
}

function FixButton({
  row,
  onOpenFix,
}: {
  row: ProductRow;
  onOpenFix?: ((row: ProductRow) => void) | undefined;
}) {
  if (!onOpenFix || !row.id || !needsFix(row)) return null;
  return (
    <Button
      variant="outline"
      size="sm"
      className="h-8"
      onClick={(e) => {
        e.stopPropagation();
        onOpenFix(row);
      }}
      aria-label={`Fix ${row.sku}`}
    >
      <Wrench aria-hidden />
      Fix
    </Button>
  );
}

function AttributesCell({ row }: { row: ProductRow }) {
  if (row.detectionError) {
    return <span className="text-destructive">{row.detectionError}</span>;
  }
  if (!row.attributes?.length) {
    return <span className="text-muted-foreground">Not read yet</span>;
  }
  return (
    <ul className="flex flex-wrap gap-1.5" aria-label={`Attributes of ${row.sku}`}>
      {row.attributes.map((a) => (
        <li
          key={a.name}
          className="inline-flex items-center gap-1 rounded-md border bg-muted/50 px-2 py-0.5 text-xs"
        >
          <span className="text-muted-foreground">{a.name}</span>
          <span className="font-medium">{a.value}</span>
        </li>
      ))}
    </ul>
  );
}

type ListItem = { kind: "rejected"; row: RejectedRow } | { kind: "product"; row: ProductRow };

/** uploadOrder ranks a row by its upload; rows with no known upload sort last. */
function uploadOrder(item: ListItem): number {
  return Number(item.row.uploadId ?? 0);
}

/** matches: the search finds a row by its SKU or its product id. */
function matches(item: ListItem, query: string): boolean {
  const q = query.trim().toLowerCase();
  if (!q) return true;
  const id = item.kind === "product" ? (item.row.id ?? "") : "";
  return item.row.sku.toLowerCase().includes(q) || id === q;
}

function StatusBadge({ row }: { row: ProductRow }) {
  const s = STATUS[row.status ?? FROM_DETECTION[row.detection]];
  return <Badge variant={s.tone}>{s.label}</Badge>;
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
      className="grid gap-4 rounded-xl border bg-card p-5 shadow-sm md:grid-cols-[1fr_1fr_auto] md:items-end"
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
  errorMessage,
  uploadName = "indigo-loom-autumn.csv",
  onGenerate,
  onResume,
  onUploadLaunch,
  onOpenFix,
  rejected = [],
  onOpenRejected,
  onDiscardAll,
  discarding = false,
}: ProductsViewProps) {
  const [neutral, setNeutral] = useState(false);
  const [query, setQuery] = useState("");
  const [page, setPage] = useState(1);
  const total = products.reduce((s, p) => s + p.aiCostMicroUsd, 0);
  // Newest upload first, so a fresh upload is on page 1 however many rows
  // older uploads left behind; within an upload its refused rows come first,
  // then its products in CSV order (the sort is stable).
  const items: ListItem[] = [
    ...rejected.map((row) => ({ kind: "rejected" as const, row })),
    ...products.map((row) => ({ kind: "product" as const, row })),
  ]
    .filter((item) => matches(item, query))
    .sort((a, b) => uploadOrder(b) - uploadOrder(a));
  const pages = Math.max(1, Math.ceil(items.length / PAGE_SIZE));
  // A search or a shorter list can leave the page past the end; show the last.
  const current = Math.min(page, pages);
  const first = (current - 1) * PAGE_SIZE;
  const shown = items.slice(first, first + PAGE_SIZE);
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
              Listings follow each brand&rsquo;s voice note. Add one in Brand voice, or confirm that
              a plain, neutral voice is fine for this run.
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

      {rejected.length > 0 ? (
        <div className="flex flex-wrap items-center justify-between gap-3 rounded-lg border bg-card px-4 py-3 text-sm shadow-sm">
          <p>
            <span className="font-medium tabular-nums">{rejected.length}</span>{" "}
            {rejected.length === 1 ? "row" : "rows"} from your CSV{" "}
            {rejected.length === 1 ? "was" : "were"} not loaded and{" "}
            {rejected.length === 1 ? "is" : "are"} listed as{" "}
            <span className="font-medium text-destructive">Not loaded</span> below.
            {onOpenRejected ? " Open one to fix it, or discard them." : ""}
          </p>
          {onDiscardAll ? (
            <Button variant="outline" size="sm" disabled={discarding} onClick={onDiscardAll}>
              <Trash2 aria-hidden />
              {discarding ? "Discarding…" : `Discard all ${String(rejected.length)}`}
            </Button>
          ) : null}
        </div>
      ) : null}

      {status === "error" ? (
        <Alert variant="destructive" role="alert">
          <AlertTitle>Products could not be loaded</AlertTitle>
          <AlertDescription>
            {errorMessage ? `${errorMessage} ` : ""}Reload the page. If it keeps failing, quote{" "}
            {requestId ?? "the time it happened"} to the Catalift team.
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
      ) : status === "ready" && products.length === 0 && rejected.length === 0 ? (
        <div className="space-y-3 rounded-xl border bg-card p-8 shadow-sm">
          <h2 className="font-medium">No products yet</h2>
          <p className="max-w-prose text-muted-foreground">
            {onUploadLaunch
              ? "Upload the launch\u2019s product list and photos; products appear here as soon as the CSV is read."
              : "A seller uploads the launch\u2019s product list and photos; products appear here as soon as the CSV is read."}
          </p>
          {onUploadLaunch ? (
            <Button className="h-11" onClick={onUploadLaunch}>
              Upload a launch
            </Button>
          ) : null}
        </div>
      ) : status === "ready" ? (
        <section aria-label="Product list" className="space-y-3">
          <div className="relative max-w-sm">
            <Search
              aria-hidden
              className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground"
            />
            <Input
              type="search"
              value={query}
              onChange={(e) => {
                setQuery(e.target.value);
                setPage(1);
              }}
              placeholder="Search by SKU or product ID"
              aria-label="Search products by SKU or product ID"
              className="h-11 pl-9"
            />
          </div>

          <div className="rounded-xl border bg-card shadow-sm">
            <Table className="min-w-[68rem]">
              <TableHeader>
                <TableRow>
                  <TableHead className="sticky left-0 z-10 bg-card">SKU</TableHead>
                  <TableHead>ID</TableHead>
                  <TableHead>Brand</TableHead>
                  <TableHead>Category</TableHead>
                  <TableHead className="text-right">Price</TableHead>
                  <TableHead>Photos</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead className="min-w-80">Attributes</TableHead>
                  <TableHead className="text-right">AI cost</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {shown.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={9} className="py-8 text-center text-muted-foreground">
                      No product matches &ldquo;{query.trim()}&rdquo;.
                    </TableCell>
                  </TableRow>
                ) : null}
                {shown.map((item) =>
                  item.kind === "rejected" ? (
                    <TableRow
                      key={rejectedKey(item.row)}
                      className={onOpenRejected ? "cursor-pointer" : undefined}
                      onClick={
                        onOpenRejected
                          ? () => {
                              onOpenRejected(item.row);
                            }
                          : undefined
                      }
                    >
                      <TableCell className="sticky left-0 z-10 bg-card font-medium">
                        {item.row.sku || <span className="text-muted-foreground">No SKU</span>}
                      </TableCell>
                      <TableCell className="text-muted-foreground">
                        Row {item.row.rowNumber}
                      </TableCell>
                      <TableCell>{item.row.brand}</TableCell>
                      <TableCell>{item.row.category}</TableCell>
                      <TableCell className="text-right tabular-nums">{item.row.price}</TableCell>
                      <TableCell className="text-muted-foreground">None</TableCell>
                      <TableCell>
                        <span className="flex items-center gap-2">
                          <Badge variant="destructive">Not loaded</Badge>
                          <RejectedFix row={item.row} onOpen={onOpenRejected} />
                        </span>
                      </TableCell>
                      <TableCell className="whitespace-normal text-destructive">
                        {item.row.reason}
                        <span className="block text-xs text-muted-foreground">
                          Row {item.row.rowNumber} of {item.row.fileName}
                        </span>
                      </TableCell>
                      <TableCell className="text-right text-muted-foreground">None</TableCell>
                    </TableRow>
                  ) : (
                    <TableRow
                      key={item.row.id ?? item.row.sku}
                      className={onOpenFix && needsFix(item.row) ? "cursor-pointer" : undefined}
                      onClick={
                        onOpenFix && needsFix(item.row) && item.row.id
                          ? () => {
                              onOpenFix(item.row);
                            }
                          : undefined
                      }
                    >
                      <TableCell className="sticky left-0 z-10 bg-card font-medium">
                        {item.row.sku}
                      </TableCell>
                      <TableCell className="text-muted-foreground tabular-nums">
                        {item.row.id ?? ""}
                      </TableCell>
                      <TableCell>{item.row.brand}</TableCell>
                      <TableCell>{item.row.category}</TableCell>
                      <TableCell className="text-right tabular-nums">
                        {formatInrMinor(item.row.priceMinor)}
                      </TableCell>
                      <TableCell className="tabular-nums">
                        {item.row.imageCount === 0 ? (
                          <span className="text-destructive">None</span>
                        ) : (
                          item.row.imageCount
                        )}
                      </TableCell>
                      <TableCell>
                        <span className="flex items-center gap-2">
                          <StatusBadge row={item.row} />
                          <FixButton row={item.row} onOpenFix={onOpenFix} />
                        </span>
                      </TableCell>
                      <TableCell className="whitespace-normal">
                        <AttributesCell row={item.row} />
                      </TableCell>
                      <TableCell className="text-right tabular-nums">
                        {formatUsdMicro(item.row.aiCostMicroUsd)}
                      </TableCell>
                    </TableRow>
                  ),
                )}
              </TableBody>
              <TableFooter>
                <TableRow>
                  <TableCell colSpan={8} className="sticky left-0 bg-muted/50">
                    Total for this upload
                  </TableCell>
                  <TableCell className="text-right tabular-nums">{formatUsdMicro(total)}</TableCell>
                </TableRow>
              </TableFooter>
            </Table>
          </div>

          <nav
            aria-label="Product pages"
            className="flex flex-wrap items-center justify-between gap-3 text-sm"
          >
            <p className="text-muted-foreground tabular-nums" role="status">
              {items.length === 0
                ? "No rows"
                : `Showing ${String(first + 1)} to ${String(first + shown.length)} of ${String(items.length)}`}
            </p>
            <div className="flex items-center gap-2">
              <Button
                variant="outline"
                size="sm"
                className="h-9"
                disabled={current <= 1}
                onClick={() => {
                  setPage(current - 1);
                }}
              >
                <ChevronLeft aria-hidden />
                Previous
              </Button>
              <span className="tabular-nums">
                Page {current} of {pages}
              </span>
              <Button
                variant="outline"
                size="sm"
                className="h-9"
                disabled={current >= pages}
                onClick={() => {
                  setPage(current + 1);
                }}
              >
                Next
                <ChevronRight aria-hidden />
              </Button>
            </div>
          </nav>
        </section>
      ) : null}
    </div>
  );
}
