import { CheckCheck, ImageOff } from "lucide-react";
import { useEffect, useRef, useState } from "react";

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Sheet, SheetContent, SheetTitle } from "@/components/ui/sheet";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Textarea } from "@/components/ui/textarea";
import type { Role } from "@/features/shell/components/AppFrame";
import { formatUsdMicro } from "@/features/shell/format";

export type RuleStatus = "unchecked" | "passing" | "failing";

export interface RuleFailure {
  rule: string;
  field: string;
  message: string;
}

export interface GridRow {
  id: string;
  /** The product this channel listing belongs to; rows are grouped by it. */
  productId?: string | undefined;
  sku: string;
  channel: string;
  version: number;
  title: string;
  bullets: [string, string, string, string, string];
  description: string;
  /** The product's attributes as one line, e.g. "navy, solid, round neck". */
  attributes: string;
  /** True when a detected attribute is missing or "unknown" (reviewer triage). */
  missingAttributes?: boolean | undefined;
  /** The model's 0 to 1 confidence in the attributes; absent once corrected. */
  confidence?: number | null | undefined;
  /** The product's thumbnail (GET /v1/products/{id}/image). */
  imageUrl?: string | undefined;
  /** The product's AI cost so far, millionths of a US dollar. */
  costMicroUsd?: number | undefined;
  ruleStatus: RuleStatus;
  ruleFailures: RuleFailure[];
  approved: boolean;
  approvedAt?: string | undefined;
  approvedBy?: string | undefined;
  regeneration?: { field: string; status: "queued" | "superseded" } | undefined;
}

export interface ApproveResult {
  approved: number;
  skipped: { sku: string; channel: string; reason: "failing_rules" | "version_changed" }[];
}

/** Triage filters (reviewer flow step 2). */
export type GridFilter = "all" | "failing" | "missing" | "low_confidence" | "unapproved";

/** Below this confidence a detection is flagged for a closer look. */
export const LOW_CONFIDENCE = 0.6;

/** The rules engine's verdict on the editor's current draft. */
export interface LiveCheck {
  listingId: string;
  ruleStatus: "passing" | "failing";
  ruleFailures: RuleFailure[];
}

export interface ReviewGridViewProps {
  status: "loading" | "ready" | "error";
  role: Role;
  rows?: GridRow[];
  filter?: GridFilter;
  selectedIds?: string[];
  editingId?: string | undefined;
  conflict?: boolean;
  approveResult?: ApproveResult | undefined;
  clearedByRecheck?: { channel: string; count: number } | undefined;
  requestId?: string;
  /** The rules' verdict on the draft being typed, for the open listing. */
  liveCheck?: LiveCheck | undefined;
  /** Interaction, wired by the route; absent in the design gallery. */
  actions?: ReviewGridActions | undefined;
}

export type ListingField =
  "title" | "bullet_1" | "bullet_2" | "bullet_3" | "bullet_4" | "bullet_5" | "description";

/** The fields a reviewer changed, by API name. */
export type ListingChanges = Partial<Record<ListingField, string>>;

export interface ReviewGridActions {
  onFilterChange: (filter: GridFilter) => void;
  /** The editor's draft changed; the route re-runs the rules on it. */
  onDraft?: ((row: GridRow, changes: ListingChanges) => void) | undefined;
  onToggleSelect: (id: string, selected: boolean) => void;
  /** Selects or clears several rows at once (the select-all checkbox). */
  onSelectMany: (ids: string[], selected: boolean) => void;
  onOpen: (id: string) => void;
  onClose: () => void;
  onSave: (row: GridRow, changes: ListingChanges) => void;
  onRegenerate: (row: GridRow, field: ListingField, instruction: string) => void;
  onApprove: () => void;
  /** Approves every listing that passes its rules and is not approved yet. */
  onApproveAll: () => void;
  onReload: () => void;
  onGoToProducts: () => void;
  saving?: boolean | undefined;
}

const FIELD_LABEL: Record<ListingField, string> = {
  title: "Title",
  bullet_1: "Bullet 1",
  bullet_2: "Bullet 2",
  bullet_3: "Bullet 3",
  bullet_4: "Bullet 4",
  bullet_5: "Bullet 5",
  description: "Description",
};

const CHANNEL_NAME: Record<string, string> = {
  amazon_style: "Amazon-style",
  own_website: "Own website",
};

const SKIP_REASON = {
  failing_rules: "fails a channel rule",
  version_changed: "changed since you loaded the grid",
};

function RuleBadge({ row }: { row: GridRow }) {
  if (row.ruleStatus === "failing") {
    return <Badge variant="destructive">{row.ruleFailures.length} rule failing</Badge>;
  }
  if (row.ruleStatus === "unchecked") return <Badge variant="outline">Not checked</Badge>;
  return <Badge variant="secondary">Passes rules</Badge>;
}

function ApprovalCell({ row }: { row: GridRow }) {
  return row.approved ? (
    <span className="text-sm">
      Approved <span className="text-muted-foreground">by {row.approvedBy}</span>
    </span>
  ) : (
    <span className="text-sm text-muted-foreground">Not approved</span>
  );
}

function textOf(form: FormData, name: string): string {
  const v = form.get(name);
  return typeof v === "string" ? v : "";
}

function changesFrom(row: GridRow, form: FormData): ListingChanges {
  const now: Record<ListingField, string> = {
    title: textOf(form, "title"),
    bullet_1: textOf(form, "bullet_1"),
    bullet_2: textOf(form, "bullet_2"),
    bullet_3: textOf(form, "bullet_3"),
    bullet_4: textOf(form, "bullet_4"),
    bullet_5: textOf(form, "bullet_5"),
    description: textOf(form, "description"),
  };
  const before: Record<ListingField, string> = {
    title: row.title,
    bullet_1: row.bullets[0],
    bullet_2: row.bullets[1],
    bullet_3: row.bullets[2],
    bullet_4: row.bullets[3],
    bullet_5: row.bullets[4],
    description: row.description,
  };
  const changes: ListingChanges = {};
  for (const f of Object.keys(now) as ListingField[]) {
    if (now[f] !== before[f]) changes[f] = now[f];
  }
  return changes;
}

function Editor({
  row,
  role,
  conflict,
  actions,
  liveCheck,
}: {
  row: GridRow;
  role: Role;
  conflict: boolean;
  actions?: ReviewGridActions | undefined;
  liveCheck?: LiveCheck | undefined;
}) {
  const canEdit = role === "reviewer";
  const [field, setField] = useState<ListingField>("title");
  const [instruction, setInstruction] = useState("");
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  useEffect(
    () => () => {
      clearTimeout(timer.current);
    },
    [],
  );
  // The badge follows the draft once the rules have run on it.
  const live = liveCheck?.listingId === row.id ? liveCheck : undefined;
  const failures = live ? live.ruleFailures : row.ruleFailures;
  const verdict: GridRow["ruleStatus"] = live ? live.ruleStatus : row.ruleStatus;
  return (
    <form
      aria-labelledby="editor"
      className="space-y-4 p-5"
      onChange={(e) => {
        if (!canEdit || !actions?.onDraft) return;
        const form = e.currentTarget;
        clearTimeout(timer.current);
        timer.current = setTimeout(() => {
          actions.onDraft?.(row, changesFrom(row, new FormData(form)));
        }, 400);
      }}
      onSubmit={(e) => {
        e.preventDefault();
        const changes = changesFrom(row, new FormData(e.currentTarget));
        if (Object.keys(changes).length > 0) actions?.onSave(row, changes);
      }}
    >
      <header className="space-y-1">
        <h2 id="editor" className="font-medium">
          {row.sku} &middot; {CHANNEL_NAME[row.channel] ?? row.channel}
        </h2>
        {actions ? (
          <Button type="button" variant="ghost" size="sm" onClick={actions.onClose}>
            Close
          </Button>
        ) : null}
        <p className="text-sm text-muted-foreground">
          Version {row.version} &middot; {row.attributes}
        </p>
        <p className="flex items-center gap-2 text-sm" role="status" aria-live="polite">
          <RuleBadge row={{ ...row, ruleStatus: verdict, ruleFailures: failures }} />
          {live ? <span className="text-muted-foreground">for your unsaved changes</span> : null}
        </p>
      </header>

      {conflict ? (
        <Alert variant="destructive" role="alert">
          <AlertTitle>Someone changed this listing while you were editing</AlertTitle>
          <AlertDescription>
            Your edit was not saved, so nothing was overwritten. Reload the listing to see version{" "}
            {row.version + 1}, then make your change again.
            {actions ? (
              <Button type="button" variant="outline" className="mt-2" onClick={actions.onReload}>
                Reload the listing
              </Button>
            ) : null}
          </AlertDescription>
        </Alert>
      ) : null}

      {failures.length > 0 ? (
        <ul className="space-y-1 text-sm" aria-label="Rule failures">
          {failures.map((f) => (
            <li key={f.rule + f.field + f.message} className="text-destructive">
              {f.message}
            </li>
          ))}
        </ul>
      ) : null}

      {row.regeneration?.status === "superseded" ? (
        <Alert role="status">
          <AlertTitle>Regeneration of the {row.regeneration.field} was not applied</AlertTitle>
          <AlertDescription>
            You changed the {row.regeneration.field} while it was being rewritten, so your text was
            kept. Regenerate it again if you still want a new version.
          </AlertDescription>
        </Alert>
      ) : row.regeneration?.status === "queued" ? (
        <p className="text-sm text-muted-foreground" role="status">
          Rewriting the {row.regeneration.field}&hellip;
        </p>
      ) : null}

      <div className="space-y-2">
        <Label htmlFor="title">Title</Label>
        <Input
          id="title"
          name="title"
          defaultValue={row.title}
          readOnly={!canEdit}
          className="h-11"
        />
      </div>
      {row.bullets.map((b, i) => (
        <div key={i} className="space-y-2">
          <Label htmlFor={`bullet-${i + 1}`}>Bullet {i + 1}</Label>
          <Input
            id={`bullet-${i + 1}`}
            name={`bullet_${i + 1}`}
            defaultValue={b}
            readOnly={!canEdit}
            className="h-11"
          />
        </div>
      ))}
      <div className="space-y-2">
        <Label htmlFor="description">Description</Label>
        <Textarea
          id="description"
          name="description"
          defaultValue={row.description}
          readOnly={!canEdit}
          rows={4}
        />
      </div>

      {canEdit ? (
        <>
          <div className="space-y-2">
            <Label htmlFor="instruction">Rewrite one field with an instruction</Label>
            <div className="flex flex-wrap gap-2">
              <Select
                value={field}
                onValueChange={(v) => {
                  setField(v as ListingField);
                }}
              >
                <SelectTrigger className="h-11 w-40" aria-label="Field to rewrite">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {(Object.keys(FIELD_LABEL) as ListingField[]).map((f) => (
                    <SelectItem key={f} value={f}>
                      {FIELD_LABEL[f]}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <Input
                id="instruction"
                placeholder="e.g. shorter, and mention cotton"
                className="h-11 min-w-48 flex-1"
                value={instruction}
                onChange={(e) => {
                  setInstruction(e.target.value);
                }}
              />
            </div>
          </div>
          <div className="flex flex-wrap gap-2">
            <Button type="submit" className="h-11" disabled={actions?.saving}>
              {actions?.saving ? "Saving…" : "Save changes"}
            </Button>
            <Button
              type="button"
              variant="outline"
              className="h-11"
              disabled={instruction.trim() === "" || row.regeneration?.status === "queued"}
              onClick={() => {
                actions?.onRegenerate(row, field, instruction.trim());
                setInstruction("");
              }}
            >
              Rewrite {FIELD_LABEL[field].toLowerCase()}
            </Button>
          </div>
          <p className="text-sm text-muted-foreground">
            The rules re-run as you type. Saving stores the text and clears this listing&rsquo;s
            approval.
          </p>
        </>
      ) : (
        <p className="text-sm text-muted-foreground">
          Only reviewers can edit and approve listings.
        </p>
      )}
    </form>
  );
}

/** One product with its channel listings: one row of the grid. */
interface ProductGroup {
  key: string;
  first: GridRow;
  listings: GridRow[];
}

function groupByProduct(rows: GridRow[]): ProductGroup[] {
  const groups = new Map<string, ProductGroup>();
  for (const r of rows) {
    const key = r.productId ?? r.sku;
    const g = groups.get(key);
    if (g) g.listings.push(r);
    else groups.set(key, { key, first: r, listings: [r] });
  }
  return [...groups.values()];
}

const approvable = (r: GridRow) => r.ruleStatus === "passing" && !r.approved;

function lowConfidence(r: GridRow): boolean {
  return typeof r.confidence === "number" && r.confidence < LOW_CONFIDENCE;
}

function matches(g: ProductGroup, filter: GridFilter): boolean {
  switch (filter) {
    case "failing":
      return g.listings.some((l) => l.ruleStatus === "failing");
    case "missing":
      return g.first.missingAttributes === true;
    case "low_confidence":
      return lowConfidence(g.first);
    case "unapproved":
      return g.listings.some((l) => !l.approved);
    case "all":
      return true;
  }
}

const FILTERS: { value: GridFilter; label: string }[] = [
  { value: "all", label: "All" },
  { value: "failing", label: "Compliance errors" },
  { value: "missing", label: "Missing attributes" },
  { value: "low_confidence", label: "Low confidence" },
  { value: "unapproved", label: "Not approved" },
];

function checkState(ids: string[], selected: string[]): boolean | "indeterminate" {
  const n = ids.filter((id) => selected.includes(id)).length;
  return n === 0 ? false : n === ids.length ? true : "indeterminate";
}

function Thumb({ row }: { row: GridRow }) {
  return row.imageUrl ? (
    <img
      src={row.imageUrl}
      alt={row.sku}
      loading="lazy"
      className="size-14 shrink-0 rounded-md border bg-muted object-cover"
    />
  ) : (
    <span
      aria-hidden
      className="grid size-14 shrink-0 place-items-center rounded-md border bg-muted text-muted-foreground"
    >
      <ImageOff className="size-5" />
    </span>
  );
}

function ProductCell({ g }: { g: ProductGroup }) {
  const r = g.first;
  return (
    <div className="flex items-start gap-3">
      <Thumb row={r} />
      <div className="min-w-0 flex-1 space-y-1">
        <p className="font-medium break-words">{r.sku}</p>
        <div className="flex flex-wrap gap-1">
          {r.costMicroUsd !== undefined ? (
            <Badge variant="outline" className="tabular-nums">
              AI {formatUsdMicro(r.costMicroUsd)}
            </Badge>
          ) : null}
          {lowConfidence(r) ? (
            <Badge variant="destructive">
              Low confidence {Math.round((r.confidence ?? 0) * 100)}%
            </Badge>
          ) : null}
          {r.missingAttributes ? <Badge variant="destructive">Missing attributes</Badge> : null}
        </div>
        <p className="text-xs break-words text-muted-foreground">
          {r.attributes || "Not detected"}
        </p>
      </div>
    </div>
  );
}

function ChannelCell({
  l,
  reviewer,
  selectedIds,
  actions,
}: {
  l: GridRow | undefined;
  reviewer: boolean;
  selectedIds: string[];
  actions?: ReviewGridActions | undefined;
}) {
  if (!l) return <span className="text-sm text-muted-foreground">No listing</span>;
  const name = CHANNEL_NAME[l.channel] ?? l.channel;
  return (
    <div className="space-y-1.5">
      <div className="flex items-start gap-2">
        {reviewer ? (
          <Checkbox
            className="mt-0.5"
            aria-label={`Select ${l.sku} on ${name}`}
            checked={selectedIds.includes(l.id)}
            onCheckedChange={(v) => actions?.onToggleSelect(l.id, v === true)}
            disabled={!approvable(l)}
          />
        ) : null}
        {actions ? (
          <Button
            variant="link"
            className="h-auto min-w-0 shrink justify-start p-0 text-left break-words whitespace-normal"
            onClick={() => {
              actions.onOpen(l.id);
            }}
          >
            {l.title || "Open"}
          </Button>
        ) : (
          <span className="min-w-0 text-sm break-words">{l.title}</span>
        )}
      </div>
      <div className="flex flex-wrap items-center gap-2">
        <RuleBadge row={l} />
        <ApprovalCell row={l} />
      </div>
    </div>
  );
}

/**
 * ReviewGridView: GET /v1/listings and GET /v1/products (the grid),
 * PATCH /v1/listings/{id}, POST /v1/rule-checks, POST .../regeneration-requests
 * and POST /v1/approvals. One row per product with a column per channel
 * (reviewer flow step 1); approval only for passing listings (Q-009).
 */
export function ReviewGridView({
  status,
  role,
  rows = [],
  filter = "all",
  selectedIds = [],
  editingId,
  conflict = false,
  approveResult,
  clearedByRecheck,
  requestId,
  liveCheck,
  actions,
}: ReviewGridViewProps) {
  const reviewer = role === "reviewer";
  const groups = groupByProduct(rows);
  const shown = groups.filter((g) => matches(g, filter));
  const channels = [...new Set(rows.map((r) => r.channel))].sort();
  const editing = rows.find((r) => r.id === editingId);
  const failing = rows.filter((r) => r.ruleStatus === "failing").length;
  const approved = rows.filter((r) => r.approved).length;
  const readyAll = rows.filter(approvable).length;
  const shownIds = shown.flatMap((g) => g.listings.filter(approvable).map((l) => l.id));
  const allShown = checkState(shownIds, selectedIds);
  const count = (f: GridFilter) => groups.filter((g) => matches(g, f)).length;

  return (
    <div className="space-y-6">
      <header className="flex flex-wrap items-end justify-between gap-4">
        <div className="space-y-1">
          <h1 className="text-2xl font-semibold tracking-tight">Review listings</h1>
          <p className="text-muted-foreground tabular-nums">
            {groups.length} products &middot; {rows.length} listings &middot; {approved} approved
            &middot; {failing} failing rules
          </p>
        </div>
        {reviewer && status === "ready" ? (
          <div className="flex flex-wrap gap-2">
            <Button
              variant="outline"
              className="h-11"
              disabled={readyAll === 0 || actions?.saving}
              onClick={actions?.onApproveAll}
            >
              <CheckCheck aria-hidden />
              Approve all passing ({readyAll})
            </Button>
            <Button
              className="h-11"
              disabled={selectedIds.length === 0 || actions?.saving}
              onClick={actions?.onApprove}
            >
              Approve {selectedIds.length} selected
            </Button>
          </div>
        ) : null}
      </header>

      {clearedByRecheck ? (
        <Alert role="status">
          <AlertTitle>{CHANNEL_NAME[clearedByRecheck.channel]} rules changed</AlertTitle>
          <AlertDescription>
            {clearedByRecheck.count} approved listings now fail the new rules and need approving
            again.
          </AlertDescription>
        </Alert>
      ) : null}

      {approveResult ? (
        <Alert role="status">
          <AlertTitle>
            {approveResult.approved} approved, {approveResult.skipped.length} skipped
          </AlertTitle>
          <AlertDescription>
            <ul className="list-disc pl-5">
              {approveResult.skipped.map((s) => (
                <li key={s.sku + s.channel}>
                  {s.sku} on {CHANNEL_NAME[s.channel]}: {SKIP_REASON[s.reason]}
                </li>
              ))}
            </ul>
          </AlertDescription>
        </Alert>
      ) : null}

      {status === "error" ? (
        <Alert variant="destructive" role="alert">
          <AlertTitle>Listings could not be loaded</AlertTitle>
          <AlertDescription>
            Reload the page. If it keeps failing, quote {requestId} to the Catalift team.
          </AlertDescription>
        </Alert>
      ) : null}

      {status === "loading" ? (
        <div className="space-y-2" aria-busy="true" aria-label="Loading listings">
          {Array.from({ length: 8 }, (_, i) => (
            <Skeleton key={i} className="h-12 w-full" />
          ))}
        </div>
      ) : status === "ready" && rows.length === 0 ? (
        <div className="space-y-3 rounded-xl border bg-card p-8 shadow-sm">
          <h2 className="font-medium">Nothing to review yet</h2>
          <p className="max-w-prose text-muted-foreground">
            Listings appear here as enrichment writes them. Start it from Products.
          </p>
          <Button className="h-11" variant="outline" onClick={actions?.onGoToProducts}>
            Go to products
          </Button>
        </div>
      ) : status === "ready" ? (
        <div>
          <div className="min-w-0 space-y-3">
            <Tabs
              value={filter}
              onValueChange={(v) => {
                actions?.onFilterChange(v as GridFilter);
              }}
            >
              <TabsList
                aria-label="Filter products"
                className="w-full justify-start overflow-x-auto overflow-y-hidden sm:w-auto"
              >
                {FILTERS.map((f) => (
                  <TabsTrigger key={f.value} value={f.value} className="flex-none">
                    {f.label}
                    {f.value === "all" ? null : (
                      <span className="ml-1 text-muted-foreground tabular-nums">
                        {count(f.value)}
                      </span>
                    )}
                  </TabsTrigger>
                ))}
              </TabsList>
            </Tabs>

            {reviewer && shownIds.length > 0 ? (
              <div className="flex flex-wrap items-center gap-3 rounded-lg border bg-card px-3 py-2 text-sm shadow-sm">
                <Checkbox
                  id="select-all"
                  className="size-5"
                  checked={allShown}
                  onCheckedChange={(v) => actions?.onSelectMany(shownIds, v === true)}
                  aria-label="Select every passing listing shown"
                />
                <label htmlFor="select-all" className="cursor-pointer">
                  Select all passing ({shownIds.length})
                </label>
                {selectedIds.length > 0 ? (
                  <>
                    <span className="text-muted-foreground tabular-nums">
                      &middot; {selectedIds.length} selected
                    </span>
                    <Button
                      variant="link"
                      size="sm"
                      className="h-auto p-0"
                      onClick={() => actions?.onSelectMany(selectedIds, false)}
                    >
                      Clear
                    </Button>
                  </>
                ) : null}
              </div>
            ) : null}

            {shown.length === 0 ? (
              <p className="rounded-xl border bg-card p-6 text-muted-foreground shadow-sm">
                No products match this filter.
              </p>
            ) : (
              <>
                <div className="hidden overflow-x-auto rounded-xl border bg-card shadow-sm md:block">
                  <Table className="min-w-[44rem] table-fixed">
                    <TableHeader>
                      <TableRow>
                        {reviewer ? (
                          <TableHead className="w-12">
                            <Checkbox
                              checked={allShown}
                              disabled={shownIds.length === 0}
                              onCheckedChange={(v) => actions?.onSelectMany(shownIds, v === true)}
                              aria-label="Select every passing listing in the table"
                            />
                          </TableHead>
                        ) : null}
                        <TableHead className="w-64">Product</TableHead>
                        {channels.map((c) => (
                          <TableHead key={c}>{CHANNEL_NAME[c] ?? c}</TableHead>
                        ))}
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {shown.map((g) => {
                        const ids = g.listings.filter(approvable).map((l) => l.id);
                        return (
                          <TableRow
                            key={g.key}
                            data-state={
                              g.listings.some((l) => l.id === editingId) ? "selected" : undefined
                            }
                          >
                            {reviewer ? (
                              <TableCell className="align-top">
                                <Checkbox
                                  aria-label={`Select every passing listing of ${g.first.sku}`}
                                  checked={checkState(ids, selectedIds)}
                                  disabled={ids.length === 0}
                                  onCheckedChange={(v) => actions?.onSelectMany(ids, v === true)}
                                />
                              </TableCell>
                            ) : null}
                            <TableCell className="align-top whitespace-normal">
                              <ProductCell g={g} />
                            </TableCell>
                            {channels.map((c) => (
                              <TableCell key={c} className="align-top whitespace-normal">
                                <ChannelCell
                                  l={g.listings.find((l) => l.channel === c)}
                                  reviewer={reviewer}
                                  selectedIds={selectedIds}
                                  actions={actions}
                                />
                              </TableCell>
                            ))}
                          </TableRow>
                        );
                      })}
                    </TableBody>
                  </Table>
                </div>
                <ul className="space-y-2 md:hidden" aria-label="Products">
                  {shown.map((g) => (
                    <li key={g.key} className="space-y-3 rounded-xl border bg-card p-3 shadow-sm">
                      <ProductCell g={g} />
                      {g.listings.map((l) => (
                        <div key={l.id} className="space-y-1 border-t pt-2">
                          <p className="text-xs font-medium text-muted-foreground">
                            {CHANNEL_NAME[l.channel] ?? l.channel}
                          </p>
                          <ChannelCell
                            l={l}
                            reviewer={reviewer}
                            selectedIds={selectedIds}
                            actions={actions}
                          />
                        </div>
                      ))}
                    </li>
                  ))}
                </ul>
              </>
            )}
          </div>
          {/* The editor slides over the grid, which keeps its full width. */}
          <Sheet
            open={editing !== undefined}
            onOpenChange={(open) => {
              if (!open) actions?.onClose();
            }}
          >
            <SheetContent
              side="right"
              showCloseButton={false}
              aria-describedby={undefined}
              className="w-full gap-0 overflow-y-auto p-0 sm:max-w-xl"
            >
              <SheetTitle className="sr-only">
                {editing
                  ? `Edit ${editing.sku} on ${CHANNEL_NAME[editing.channel] ?? editing.channel}`
                  : "Edit listing"}
              </SheetTitle>
              {editing ? (
                <Editor
                  key={`${editing.id}:${String(editing.version)}`}
                  row={editing}
                  role={role}
                  conflict={conflict}
                  actions={actions}
                  liveCheck={liveCheck}
                />
              ) : null}
            </SheetContent>
          </Sheet>
        </div>
      ) : null}
    </div>
  );
}
