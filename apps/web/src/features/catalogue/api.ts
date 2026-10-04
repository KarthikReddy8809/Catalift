import { queryOptions } from "@tanstack/react-query";
import { z } from "zod";

import { apiFetch, apiSend } from "@/lib/api";
import { fetchAllPages } from "@/lib/paged";

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
  sku: z.string(),
  brand: z.object({ id: z.string(), name: z.string() }),
  category: z.string(),
  price_minor: z.number(),
  currency: z.string(),
  image_count: z.number(),
  attributes: z.object({
    detection_status: detectionStatus,
    revision: z.number(),
    colour: z.string().nullable(),
    pattern: z.string().nullable(),
    sleeve: z.string().nullable(),
    neckline: z.string().nullable(),
    fit: z.string().nullable(),
    detection_error: z.string().nullable(),
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
  brandsWithoutVoice: () => [...catalogueKeys.all, "brands-without-voice"] as const,
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
export function brandsWithoutVoiceQueryOptions() {
  return queryOptions({
    queryKey: catalogueKeys.brandsWithoutVoice(),
    queryFn: async ({ signal }) =>
      (await fetchAllPages("/v1/brands", brandSchema, signal))
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
