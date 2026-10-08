import { queryOptions } from "@tanstack/react-query";
import { z } from "zod";

import { apiSend } from "@/lib/api";
import { fetchAllPages } from "@/lib/paged";
import { uploadQuery } from "@/lib/upload-filter";

export const brandSchema = z.object({
  id: z.string(),
  name: z.string(),
  /** The brand's tone. */
  voice_note: z.string().nullable(),
  // v1.6: words or phrases the brand never uses (ADR-0012).
  words_to_avoid: z.array(z.string()).default([]),
  updated_at: z.string(),
});

export type Brand = z.infer<typeof brandSchema>;

export const brandKeys = {
  all: ["brands"] as const,
  list: (uploadId?: string) => [...brandKeys.all, "list", uploadId ?? "all"] as const,
};

/** brandsQueryOptions lists the brands of one upload, or of every upload. */
export function brandsQueryOptions(uploadId?: string) {
  return queryOptions({
    queryKey: brandKeys.list(uploadId),
    queryFn: ({ signal }) =>
      fetchAllPages(uploadQuery("/v1/brands", uploadId), brandSchema, signal),
  });
}

export interface BrandVoice {
  tone: string;
  wordsToAvoid: string[];
}

/** saveBrandVoice sets the tone (empty clears it) and the words to avoid. */
export function saveBrandVoice(id: string, v: BrandVoice): Promise<Brand> {
  const tone = v.tone.trim();
  return apiSend(`/v1/brands/${id}`, brandSchema, {
    method: "PATCH",
    body: { voice_note: tone === "" ? null : tone, words_to_avoid: v.wordsToAvoid },
  });
}
