import { queryOptions } from "@tanstack/react-query";
import { z } from "zod";

import { apiFetch, apiSend } from "@/lib/api";
import { fetchAllPages } from "@/lib/paged";
import { uploadQuery } from "@/lib/upload-filter";

const detectionStatus = z.enum(["pending", "done", "failed", "stopped_budget"]);

export const uploadSchema = z.object({
  id: z.string(),
  file_name: z.string(),
  rows_total: z.number(),
  rows_accepted: z.number(),
  rows_rejected: z.number(),
  row_errors: z.array(
    z.object({ row_number: z.number(), sku: z.string().nullable(), reason: z.string() }),
  ),
  ai_cost_micro_usd: z.number(),
  created_at: z.string(),
});

export type Upload = z.infer<typeof uploadSchema>;

export const imagesResultSchema = z.object({
  attached: z.array(z.object({ file_name: z.string(), sku: z.string(), position: z.number() })),
  unmatched_files: z.array(z.string()),
  rejected_files: z.array(z.object({ file_name: z.string(), reason: z.string() })),
  products_missing_image: z.array(z.string()),
});

export type ImagesResult = z.infer<typeof imagesResultSchema>;

export const productSchema = z.object({
  id: z.string(),
  // v1.6; optional so an older API still lists products.
  upload_id: z.string().optional(),
  sku: z.string(),
  brand: z.object({ id: z.string(), name: z.string() }),
  category: z.string(),
  price_minor: z.number(),
  currency: z.string(),
  image_count: z.number(),
  // Optional so an API one release behind still lists products; the page
  // then derives the status from detection.
  status: z
    .enum([
      "needs_photo",
      "uploaded",
      "enriching",
      "ready_for_review",
      "approved",
      "failed",
      "budget_exhausted",
    ])
    .optional(),
  listings: z
    .object({ total: z.number(), failing_rules: z.number(), approved: z.number() })
    .optional(),
  attributes: z.object({
    detection_status: detectionStatus,
    revision: z.number(),
    colour: z.string().nullable(),
    pattern: z.string().nullable(),
    sleeve: z.string().nullable(),
    neckline: z.string().nullable(),
    fit: z.string().nullable(),
    detection_error: z.string().nullable(),
    confidence: z.number().nullable().optional(),
  }),
  ai_cost_micro_usd: z.number(),
  created_at: z.string(),
});

export type Product = z.infer<typeof productSchema>;

const counts = z.object({
  pending: z.number().optional(),
  done: z.number().optional(),
  queued: z.number().optional(),
  generated: z.number().optional(),
  failed: z.number(),
  stopped_budget: z.number(),
});

export const runSchema = z.object({
  id: z.string(),
  created_at: z.string(),
  detection: counts,
  listings: counts,
});

export type Run = z.infer<typeof runSchema>;

export const catalogueKeys = {
  all: ["catalogue"] as const,
  upload: (id: string) => [...catalogueKeys.all, "upload", id] as const,
  products: (uploadId: string | undefined) =>
    [...catalogueKeys.all, "products", uploadId ?? "all"] as const,
  brandsWithoutVoice: (uploadId?: string) =>
    [...catalogueKeys.all, "brands-without-voice", uploadId ?? "all"] as const,
  latestUpload: () => [...catalogueKeys.all, "latest-upload"] as const,
  run: (id: string) => [...catalogueKeys.all, "run", id] as const,
};

export function uploadQueryOptions(id: string) {
  return queryOptions({
    queryKey: catalogueKeys.upload(id),
    queryFn: ({ signal }) => apiFetch(`/v1/uploads/${id}`, uploadSchema, { signal }),
  });
}

export function productsQueryOptions(uploadId: string | undefined) {
  return queryOptions({
    queryKey: catalogueKeys.products(uploadId),
    queryFn: ({ signal }) =>
      fetchAllPages(
        uploadId ? `/v1/products?filter[upload_id]=${uploadId}` : "/v1/products",
        productSchema,
        signal,
      ),
  });
}

const brandSchema = z.object({
  id: z.string(),
  name: z.string(),
  voice_note: z.string().nullable(),
});

/** Brands with no voice note, so the run's neutral-voice question is asked up front. */
export function brandsWithoutVoiceQueryOptions(uploadId?: string) {
  return queryOptions({
    queryKey: catalogueKeys.brandsWithoutVoice(uploadId),
    queryFn: async ({ signal }) =>
      (await fetchAllPages(uploadQuery("/v1/brands", uploadId), brandSchema, signal))
        .filter((b) => b.voice_note === null)
        .map((b) => b.name),
  });
}

/** runQueryOptions polls while work is queued (AC-US-00-003-6). */
export function runQueryOptions(id: string) {
  return queryOptions({
    queryKey: catalogueKeys.run(id),
    queryFn: ({ signal }) => apiFetch(`/v1/generation-runs/${id}`, runSchema, { signal }),
    refetchInterval: (q) => {
      const r = q.state.data;
      return r && (r.detection.pending ?? 0) + (r.listings.queued ?? 0) > 0 ? 2_000 : false;
    },
  });
}

export function uploadCsv(file: File): Promise<Upload> {
  const body = new FormData();
  body.append("file", file);
  return apiSend("/v1/uploads", uploadSchema, { method: "POST", body, idempotent: true });
}

/** Photos go up in batches of 10, the API's per-request cap being 20. */
export async function uploadImages(uploadId: string, files: File[]): Promise<ImagesResult> {
  const total: ImagesResult = {
    attached: [],
    unmatched_files: [],
    rejected_files: [],
    products_missing_image: [],
  };
  for (let i = 0; i < files.length; i += 10) {
    const body = new FormData();
    for (const f of files.slice(i, i + 10)) body.append("files", f);
    const res = await apiSend(`/v1/uploads/${uploadId}/images`, imagesResultSchema, {
      method: "POST",
      body,
      idempotent: true,
    });
    total.attached.push(...res.attached);
    total.unmatched_files.push(...res.unmatched_files);
    total.rejected_files.push(...res.rejected_files);
    total.products_missing_image = res.products_missing_image;
  }
  return total;
}

export function startRun(neutral: boolean, uploadId: string | undefined): Promise<Run> {
  return apiSend("/v1/generation-runs", runSchema, {
    method: "POST",
    body: { neutral_voice_confirmed: neutral, ...(uploadId ? { upload_id: uploadId } : {}) },
    idempotent: true,
  });
}

export function resumeRun(id: string): Promise<Run> {
  return apiSend(`/v1/generation-runs/${id}/resume`, runSchema, { method: "POST" });
}

export const rowErrorSchema = z.object({
  upload_id: z.string(),
  file_name: z.string(),
  row_number: z.number(),
  sku: z.string().nullable(),
  reason: z.string(),
  category: z.string().nullable(),
  brand: z.string().nullable(),
  price: z.string().nullable(),
});

export type RowErrorItem = z.infer<typeof rowErrorSchema>;

const rowErrorsSchema = z.object({
  data: z.array(rowErrorSchema),
  categories: z.array(z.string()),
});

/** Rejected CSV rows not yet fixed, and the categories a fix may use. */
export function rowErrorsQueryOptions(uploadId: string | undefined) {
  return queryOptions({
    queryKey: [...catalogueKeys.all, "row-errors", uploadId ?? "all"] as const,
    queryFn: ({ signal }) =>
      apiFetch(
        uploadId ? `/v1/row-errors?filter[upload_id]=${uploadId}` : "/v1/row-errors",
        rowErrorsSchema,
        { signal },
      ),
  });
}

export const rowFixSchema = z.object({
  status: z.enum(["loaded", "rejected"]),
  reason: z.string().nullable(),
  product_id: z.string().nullable(),
});

export interface RowValues {
  sku: string;
  category: string;
  brand: string;
  price: string;
}

/** fixRow sends a corrected rejected row; it is loaded, or refused with a new reason. */
export function fixRow(uploadId: string, rowNumber: number, values: RowValues) {
  return apiSend(`/v1/uploads/${uploadId}/rows/${String(rowNumber)}`, rowFixSchema, {
    method: "PUT",
    body: values,
  });
}

/** uploadProductPhotos adds photos to one product, whatever the files are called. */
export function uploadProductPhotos(productId: string, files: File[]): Promise<ImagesResult> {
  const body = new FormData();
  for (const f of files) body.append("files", f);
  return apiSend(`/v1/products/${productId}/images`, imagesResultSchema, {
    method: "POST",
    body,
    idempotent: true,
  });
}

const productSaveSchema = z.object({
  status: z.enum(["saved", "rejected"]),
  reason: z.string().nullable(),
});

/** updateProduct corrects a loaded product's fields; a refused change says why. */
export function updateProduct(productId: string, values: RowValues) {
  return apiSend(`/v1/products/${productId}`, productSaveSchema, { method: "PATCH", body: values });
}

/** enrichProduct runs the one vision call again for this product only. */
export function enrichProduct(productId: string): Promise<Run> {
  return apiSend(`/v1/products/${productId}/enrich`, runSchema, {
    method: "POST",
    idempotent: true,
  });
}

/** discardRow drops one rejected row the seller does not want to load. */
export function discardRow(uploadId: string, rowNumber: number) {
  return apiSend(`/v1/uploads/${uploadId}/rows/${String(rowNumber)}`, z.undefined(), {
    method: "DELETE",
  });
}

/** discardRows drops every open rejected row, or one upload's. */
export function discardRows(uploadId: string | undefined) {
  return apiSend(
    uploadId ? `/v1/row-errors?filter[upload_id]=${uploadId}` : "/v1/row-errors",
    z.object({ discarded: z.number() }),
    { method: "DELETE" },
  );
}
