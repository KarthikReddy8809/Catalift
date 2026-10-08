import { useMutation, useQuery, useQueryClient, useSuspenseQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import { sessionQueryOptions } from "@/features/auth/api";
import { productsQueryOptions, type Product } from "@/features/catalogue/api";
import { useCurrentUpload } from "@/features/catalogue/current";
import { ApiError, errorBody } from "@/lib/api";
import { env } from "@/lib/env";

import {
  approve,
  checkDraft,
  listingKeys,
  listingsQueryOptions,
  regenerate,
  saveListing,
  type Listing,
} from "../api";
import {
  ReviewGridView,
  type ApproveResult,
  type GridFilter,
  type GridRow,
  type LiveCheck,
  type ReviewGridViewProps,
} from "../components/ReviewGridView";

function toRow(l: Listing, p: Product | undefined): GridRow {
  const b = l.bullets ?? [];
  const a = l.attributes;
  const reg = l.latest_regeneration;
  return {
    id: l.id,
    sku: l.sku,
    channel: l.channel,
    version: l.version,
    title:
      l.title ?? (l.status === "generated" ? "" : `Not written: ${l.failure_reason ?? l.status}`),
    bullets: [b[0] ?? "", b[1] ?? "", b[2] ?? "", b[3] ?? "", b[4] ?? ""],
    description: l.description ?? "",
    attributes: [a.colour, a.pattern, a.sleeve, a.neckline, a.fit]
      .filter((v): v is string => v !== null && v !== "unknown")
      .join(", "),
    productId: l.product_id,
    missingAttributes: [a.colour, a.pattern, a.sleeve, a.neckline, a.fit].some(
      (v) => v === null || v === "unknown",
    ),
    confidence: p?.attributes.confidence,
    costMicroUsd: p?.ai_cost_micro_usd,
    imageUrl:
      p && p.image_count > 0 ? `${env.VITE_API_URL}/v1/products/${l.product_id}/image` : undefined,
    ruleStatus: l.rule_status,
    ruleFailures: l.rule_failures,
    approved: l.approved,
    approvedAt: l.approved_at ?? undefined,
    approvedBy: l.approved_by ?? undefined,
    regeneration:
      reg && (reg.status === "queued" || reg.status === "superseded")
        ? { field: reg.field, status: reg.status }
        : undefined,
  };
}

/** ReviewPage: the grid, the editor, regeneration and bulk approval (US-00-007 to -009). */
export function ReviewPage() {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { data: session } = useSuspenseQuery(sessionQueryOptions());
  // Only the newest upload's listings; older ones stay in the database.
  const current = useCurrentUpload();
  const listings = useQuery({ ...listingsQueryOptions(current.id), enabled: current.ready });
  const products = useQuery({ ...productsQueryOptions(current.id), enabled: current.ready });
  const [filter, setFilter] = useState<GridFilter>("all");
  const [liveCheck, setLiveCheck] = useState<LiveCheck | undefined>();
  const [selected, setSelected] = useState<string[]>([]);
  const [editingId, setEditingId] = useState<string | undefined>();
  const [conflict, setConflict] = useState(false);
  const [approveResult, setApproveResult] = useState<ApproveResult | undefined>();

  const refresh = () => queryClient.invalidateQueries({ queryKey: listingKeys.all });
  const save = useMutation({
    mutationFn: (v: { row: GridRow; changes: Parameters<typeof saveListing>[2] }) =>
      saveListing(v.row.id, v.row.version, v.changes),
    onMutate: () => {
      setConflict(false);
    },
    onSuccess: () => {
      setLiveCheck(undefined);
      void refresh();
    },
    onError: (err) => {
      if (err instanceof ApiError && err.status === 409) setConflict(true);
    },
  });
  const regen = useMutation({
    mutationFn: (v: { id: string; field: Parameters<typeof regenerate>[1]; instruction: string }) =>
      regenerate(v.id, v.field, v.instruction),
    onSuccess: () => {
      // A rewrite is one more AI call, so the product's cost moves too.
      void refresh();
      void queryClient.invalidateQueries({ queryKey: ["catalogue"] });
    },
  });
  const approval = useMutation({
    mutationFn: approve,
    onSuccess: (res) => {
      const byId = new Map((listings.data ?? []).map((l) => [l.id, l]));
      setApproveResult({
        approved: res.approved.length,
        skipped: res.skipped
          .filter((s) => s.reason === "failing_rules" || s.reason === "version_changed")
          .map((s) => ({
            sku: byId.get(s.listing_id)?.sku ?? s.listing_id,
            channel: byId.get(s.listing_id)?.channel ?? "",
            reason: s.reason as "failing_rules" | "version_changed",
          })),
      });
      setSelected([]);
      void refresh();
    },
  });

  const byProduct = new Map((products.data ?? []).map((p) => [p.id, p]));
  const rows = (listings.data ?? []).map((l) => toRow(l, byProduct.get(l.product_id)));
  const draft = useMutation({
    mutationFn: (v: { id: string; changes: Parameters<typeof checkDraft>[1] }) =>
      checkDraft(v.id, v.changes),
    onSuccess: (res, v) => {
      setLiveCheck({
        listingId: v.id,
        ruleStatus: res.rule_status,
        ruleFailures: res.rule_failures,
      });
    },
  });
  const failure = listings.error ?? save.error ?? regen.error ?? approval.error;
  const props: ReviewGridViewProps = {
    status: listings.isPending ? "loading" : listings.isError ? "error" : "ready",
    role: session?.user.role ?? "seller",
    rows,
    filter,
    selectedIds: selected,
    editingId,
    conflict,
    approveResult,
    liveCheck,
    actions: {
      onFilterChange: setFilter,
      onToggleSelect: (id, on) => {
        setSelected((cur) => (on ? [...new Set([...cur, id])] : cur.filter((x) => x !== id)));
      },
      onSelectMany: (ids, on) => {
        setSelected((cur) =>
          on ? [...new Set([...cur, ...ids])] : cur.filter((x) => !ids.includes(x)),
        );
      },
      onOpen: (id) => {
        setConflict(false);
        setEditingId(id);
      },
      onClose: () => {
        setEditingId(undefined);
      },
      onDraft: (row, changes) => {
        draft.mutate({ id: row.id, changes });
      },
      onSave: (row, changes) => {
        save.mutate({ row, changes });
      },
      onRegenerate: (row, field, instruction) => {
        regen.mutate({ id: row.id, field, instruction });
      },
      onApprove: () => {
        const byId = new Map(rows.map((r) => [r.id, r]));
        approval.mutate(
          selected.flatMap((id) => {
            const r = byId.get(id);
            return r ? [{ listing_id: id, version: r.version }] : [];
          }),
        );
      },
      onApproveAll: () => {
        approval.mutate(
          rows
            .filter((r) => r.ruleStatus === "passing" && !r.approved)
            .map((r) => ({ listing_id: r.id, version: r.version })),
        );
      },
      onReload: () => {
        setConflict(false);
        void refresh();
      },
      onGoToProducts: () => {
        void navigate({ to: "/products" });
      },
      saving: save.isPending || approval.isPending,
    },
  };
  const body = errorBody(failure);
  if (body?.requestId) props.requestId = body.requestId;
  return (
    <>
      <ReviewGridView {...props} />
      {(regen.isError || approval.isError || (save.isError && !conflict)) && body ? (
        <p role="alert" className="mt-4 text-sm text-destructive">
          {body.message}
        </p>
      ) : null}
    </>
  );
}
