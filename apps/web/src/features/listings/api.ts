import { queryOptions } from "@tanstack/react-query";
import { z } from "zod";

import { apiSend } from "@/lib/api";
import { fetchAllPages } from "@/lib/paged";

import type { ListingChanges, ListingField } from "./components/ReviewGridView";

const text = z.string().nullable();

export const listingSchema = z.object({
  id: z.string(),
  product_id: z.string(),
  sku: z.string(),
  channel: z.string(),
  status: z.enum(["queued", "generated", "failed", "stopped_budget"]),
  failure_reason: text,
  version: z.number(),
  title: text,
  bullets: z.array(z.string()).nullable(),
  description: text,
  rule_status: z.enum(["unchecked", "passing", "failing"]),
  rule_failures: z.array(z.object({ rule: z.string(), field: z.string(), message: z.string() })),
  approved: z.boolean(),
  approved_at: text,
  approved_by: text,
  attributes: z.object({ colour: text, pattern: text, sleeve: text, neckline: text, fit: text }),
  attributes_revision: z.number(),
  latest_regeneration: z
    .object({
      field: z.string(),
      status: z.enum(["queued", "applied", "superseded", "failed", "stopped_budget"]),
    })
    .nullable(),
});

export type Listing = z.infer<typeof listingSchema>;

export const listingKeys = { all: ["listings"] as const };

export function listingsQueryOptions() {
  return queryOptions({
    queryKey: listingKeys.all,
    queryFn: ({ signal }) => fetchAllPages("/v1/listings", listingSchema, signal, 30),
    // While a regeneration is being written the grid follows it.
    refetchInterval: (q) =>
      q.state.data?.some((l) => l.latest_regeneration?.status === "queued") ? 2_000 : false,
  });
}

export function saveListing(
  id: string,
  version: number,
  changes: ListingChanges,
): Promise<Listing> {
  return apiSend(`/v1/listings/${id}`, listingSchema, {
    method: "PATCH",
    body: { version, ...changes },
  });
}

export function regenerate(id: string, field: ListingField, instruction: string): Promise<unknown> {
  return apiSend(`/v1/listings/${id}/regeneration-requests`, z.unknown(), {
    method: "POST",
    body: { field, instruction },
    idempotent: true,
  });
}

export const approvalResultSchema = z.object({
  approved: z.array(
    z.object({ listing_id: z.string(), version: z.number(), approved_at: z.string() }),
  ),
  skipped: z.array(z.object({ listing_id: z.string(), reason: z.string() })),
});

export type ApprovalResult = z.infer<typeof approvalResultSchema>;

export function approve(items: { listing_id: string; version: number }[]): Promise<ApprovalResult> {
  return apiSend("/v1/approvals", approvalResultSchema, {
    method: "POST",
    body: { items },
    idempotent: true,
  });
}
