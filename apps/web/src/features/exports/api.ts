import { queryOptions } from "@tanstack/react-query";
import { z } from "zod";

import { apiFetch, apiSend } from "@/lib/api";
import { env } from "@/lib/env";
import { uploadQuery } from "@/lib/upload-filter";

export const exportSchema = z.object({
  id: z.string(),
  created_at: z.string(),
  files: z.array(
    z.object({ channel: z.string(), row_count: z.number(), download_url: z.string() }),
  ),
  skipped_channels: z.array(z.object({ channel: z.string(), reason: z.string() })),
  // v1.6: set once a reviewer sends the export to the seller (ADR-0012).
  sent_at: z.string().nullable().default(null),
  sent_by: z.string().nullable().default(null),
});

export type Export = z.infer<typeof exportSchema>;

const exportPageSchema = z.object({ data: z.array(exportSchema) });

export const exportKeys = {
  all: ["exports"] as const,
  list: (uploadId?: string) => [...exportKeys.all, "list", uploadId ?? "all"] as const,
};

/** createExport writes one upload's approved listings, or every upload's without an id. */
export function createExport(uploadId?: string): Promise<Export> {
  return apiSend("/v1/exports", exportSchema, {
    method: "POST",
    idempotent: true,
    ...(uploadId ? { body: { upload_id: uploadId } } : {}),
  });
}

/**
 * exportsQueryOptions lists the newest exports. The API decides what each
 * role sees: a reviewer every export, a seller only those sent to them.
 */
export function exportsQueryOptions(uploadId?: string) {
  return queryOptions({
    queryKey: exportKeys.list(uploadId),
    queryFn: async ({ signal }) =>
      (await apiFetch(uploadQuery("/v1/exports?limit=100", uploadId), exportPageSchema, { signal }))
        .data,
  });
}

/** sendExport hands an export to the seller; sending again changes nothing. */
export function sendExport(id: string): Promise<Export> {
  return apiSend(`/v1/exports/${id}/send`, exportSchema, { method: "POST" });
}

/** downloadHref makes an API download path absolute against VITE_API_URL. */
export function downloadHref(path: string): string {
  return `${env.VITE_API_URL}${path}`;
}
