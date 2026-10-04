import { z } from "zod";

import { apiSend } from "@/lib/api";
import { env } from "@/lib/env";

export const exportSchema = z.object({
  id: z.string(),
  created_at: z.string(),
  files: z.array(
    z.object({ channel: z.string(), row_count: z.number(), download_url: z.string() }),
  ),
  skipped_channels: z.array(z.object({ channel: z.string(), reason: z.string() })),
});

export type Export = z.infer<typeof exportSchema>;

export function createExport(): Promise<Export> {
  return apiSend("/v1/exports", exportSchema, { method: "POST", idempotent: true });
}

/** downloadHref makes an API download path absolute against VITE_API_URL. */
export function downloadHref(path: string): string {
  return `${env.VITE_API_URL}${path}`;
}
