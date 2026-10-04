import { useState } from "react";

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
import { cn } from "@/lib/utils";

export type RuleStatus = "unchecked" | "passing" | "failing";

export interface RuleFailure {
  rule: string;
  field: string;
  message: string;
}

export interface GridRow {
  id: string;
  sku: string;
  channel: string;
  version: number;
  title: string;
  bullets: [string, string, string, string, string];
  description: string;
  /**
   * Assumption: the API's Listing schema has no attributes (raised as a gap);
   * the design shows them as if GET /v1/listings returned them.
   */
  attributes: string;
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

export interface ReviewGridViewProps {
  status: "loading" | "ready" | "error";
  role: Role;
  rows?: GridRow[];
  filter?: "all" | "failing" | "unapproved";
  selectedIds?: string[];
  editingId?: string | undefined;
  conflict?: boolean;
  approveResult?: ApproveResult | undefined;
  clearedByRecheck?: { channel: string; count: number } | undefined;
  requestId?: string;
  /** Interaction, wired by the route; absent in the design gallery. */
  actions?: ReviewGridActions | undefined;
}

export type ListingField =
  "title" | "bullet_1" | "bullet_2" | "bullet_3" | "bullet_4" | "bullet_5" | "description";

/** The fields a reviewer changed, by API name. */
export type ListingChanges = Partial<Record<ListingField, string>>;

export interface ReviewGridActions {
  onFilterChange: (filter: "all" | "failing" | "unapproved") => void;
  onToggleSelect: (id: string, selected: boolean) => void;
  onOpen: (id: string) => void;
  onClose: () => void;
  onSave: (row: GridRow, changes: ListingChanges) => void;
  onRegenerate: (row: GridRow, field: ListingField, instruction: string) => void;
  onApprove: () => void;
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
}: {
  row: GridRow;
  role: Role;
  conflict: boolean;
  actions?: ReviewGridActions | undefined;
}) {
  const canEdit = role === "reviewer";
  const [field, setField] = useState<ListingField>("title");
  const [instruction, setInstruction] = useState("");
  return (
    <form
      aria-labelledby="editor"
      className="space-y-4 rounded-lg border p-4"
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

      {row.ruleFailures.length > 0 ? (
        <ul className="space-y-1 text-sm" aria-label="Rule failures">
          {row.ruleFailures.map((f) => (
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
            Saving re-checks the rules and clears this listing&rsquo;s approval.
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

/**
 * ReviewGridView: GET /v1/listings (the grid), PATCH /v1/listings/{id},
 * POST .../regeneration-requests and POST /v1/approvals. One row per product
 * and channel (AC-US-00-007-1); approval only for passing rows (Q-009).
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
  actions,
}: ReviewGridViewProps) {
  const reviewer = role === "reviewer";
  const shown = rows.filter((r) =>
    filter === "failing"
      ? r.ruleStatus === "failing"
      : filter === "unapproved"
        ? !r.approved
        : true,
  );
  const editing = rows.find((r) => r.id === editingId);
  const failing = rows.filter((r) => r.ruleStatus === "failing").length;
  const approved = rows.filter((r) => r.approved).length;

  return (
    <div className="space-y-6">
      <header className="flex flex-wrap items-end justify-between gap-4">
        <div className="space-y-1">
          <h1 className="text-2xl font-semibold tracking-tight">Review listings</h1>
          <p className="text-muted-foreground tabular-nums">
            {rows.length} listings &middot; {approved} approved &middot; {failing} failing rules
          </p>
        </div>
        {reviewer && status === "ready" ? (
          <Button
            className="h-11"
            disabled={selectedIds.length === 0 || actions?.saving}
            onClick={actions?.onApprove}
          >
            Approve {selectedIds.length} selected
          </Button>
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
        <div className="space-y-3 rounded-lg border p-8">
          <h2 className="font-medium">Nothing to review yet</h2>
          <p className="max-w-prose text-muted-foreground">
            Listings appear here as generation writes them. Start generation from Products.
          </p>
          <Button className="h-11" variant="outline" onClick={actions?.onGoToProducts}>
            Go to products
          </Button>
        </div>
      ) : status === "ready" ? (
        <div className={cn("grid gap-6", editing && "lg:grid-cols-[minmax(0,1fr)_24rem]")}>
          <div className={cn("space-y-3", editing && "hidden lg:block")}>
            <Tabs
              value={filter}
              onValueChange={(v) => {
                actions?.onFilterChange(v as "all" | "failing" | "unapproved");
              }}
            >
              <TabsList aria-label="Filter listings">
                <TabsTrigger value="all">All</TabsTrigger>
                <TabsTrigger value="failing">Failing rules</TabsTrigger>
                <TabsTrigger value="unapproved">Not approved</TabsTrigger>
              </TabsList>
            </Tabs>

            {shown.length === 0 ? (
              <p className="rounded-lg border p-6 text-muted-foreground">
                No listings match this filter. Every listing passes its channel&rsquo;s rules.
              </p>
            ) : (
              <>
                <div className="hidden md:block">
                  <Table>
                    <TableHeader>
                      <TableRow>
                        {reviewer ? (
                          <TableHead className="w-10">
                            <span className="sr-only">Select</span>
                          </TableHead>
                        ) : null}
                        <TableHead>SKU</TableHead>
                        <TableHead>Channel</TableHead>
                        <TableHead>Title</TableHead>
                        <TableHead>Rules</TableHead>
                        <TableHead>Approval</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {shown.map((r) => (
                        <TableRow
                          key={r.id}
                          data-state={r.id === editingId ? "selected" : undefined}
                        >
                          {reviewer ? (
                            <TableCell>
                              <Checkbox
                                aria-label={`Select ${r.sku} on ${CHANNEL_NAME[r.channel]}`}
                                checked={selectedIds.includes(r.id)}
                                onCheckedChange={(v) => actions?.onToggleSelect(r.id, v === true)}
                                disabled={r.ruleStatus !== "passing" || r.approved}
                              />
                            </TableCell>
                          ) : null}
                          <TableCell className="font-medium">{r.sku}</TableCell>
                          <TableCell>{CHANNEL_NAME[r.channel] ?? r.channel}</TableCell>
                          <TableCell className="max-w-80 truncate">
                            {actions ? (
                              <Button
                                variant="link"
                                className="h-auto max-w-full justify-start truncate p-0"
                                onClick={() => {
                                  actions.onOpen(r.id);
                                }}
                              >
                                {r.title || "Open"}
                              </Button>
                            ) : (
                              r.title
                            )}
                          </TableCell>
                          <TableCell>
                            <RuleBadge row={r} />
                          </TableCell>
                          <TableCell>
                            <ApprovalCell row={r} />
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </div>
                <ul className="space-y-2 md:hidden" aria-label="Listings">
                  {shown.map((r) => (
                    <li key={r.id} className="space-y-2 rounded-lg border p-3">
                      <div className="flex items-center justify-between gap-2">
                        {reviewer ? (
                          <Checkbox
                            className="size-6"
                            aria-label={`Select ${r.sku} on ${CHANNEL_NAME[r.channel]}`}
                            checked={selectedIds.includes(r.id)}
                            onCheckedChange={(v) => actions?.onToggleSelect(r.id, v === true)}
                            disabled={r.ruleStatus !== "passing" || r.approved}
                          />
                        ) : null}
                        <span className="mr-auto font-medium">
                          {r.sku} &middot; {CHANNEL_NAME[r.channel]}
                        </span>
                        <RuleBadge row={r} />
                      </div>
                      {actions ? (
                        <Button
                          variant="link"
                          className="h-auto p-0 text-left whitespace-normal"
                          onClick={() => {
                            actions.onOpen(r.id);
                          }}
                        >
                          {r.title || "Open"}
                        </Button>
                      ) : (
                        <p className="line-clamp-2 text-sm">{r.title}</p>
                      )}
                      <ApprovalCell row={r} />
                    </li>
                  ))}
                </ul>
              </>
            )}
          </div>
          {editing ? (
            <Editor
              key={`${editing.id}:${String(editing.version)}`}
              row={editing}
              role={role}
              conflict={conflict}
              actions={actions}
            />
          ) : null}
        </div>
      ) : null}
    </div>
  );
}
