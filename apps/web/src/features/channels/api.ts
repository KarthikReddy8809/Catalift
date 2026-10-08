import { queryOptions } from "@tanstack/react-query";
import { z } from "zod";

import { apiSend } from "@/lib/api";
import { fetchAllPages } from "@/lib/paged";
import { uploadQuery } from "@/lib/upload-filter";

export const channelSchema = z.object({
  id: z.string(),
  name: z.string(),
  enabled: z.boolean(),
  load_error: z.string().nullable(),
  title_max_length: z.number().nullable(),
  required_attributes: z.array(z.string()),
  banned_words: z.array(z.string()),
  export_headers: z.array(z.string()),
  last_recheck: z
    .object({
      config_hash: z.string(),
      listings_rechecked: z.number(),
      approvals_cleared: z.number(),
      created_at: z.string(),
    })
    .nullable(),
  config_hash: z.string().nullable(),
  last_edit: z.object({ by: z.string(), at: z.string() }).nullable(),
  listings_total: z.number(),
  listings_approved: z.number(),
});

export type Channel = z.infer<typeof channelSchema>;

/** channelsQueryOptions: the rules, with listing counts for one upload or every upload. */
export function channelsQueryOptions(uploadId?: string) {
  return queryOptions({
    queryKey: ["channels", uploadId ?? "all"] as const,
    queryFn: ({ signal }) =>
      fetchAllPages(uploadQuery("/v1/channels", uploadId), channelSchema, signal),
  });
}

export const rulesResultSchema = z.object({
  channel: z.string(),
  listings_rechecked: z.number(),
  approvals_cleared: z.number(),
});

/** saveRules changes a channel's rules (PATCH /v1/channels/{id}) and reports the re-check. */
export function saveRules(
  id: string,
  configHash: string,
  rules: { title_max_length: number; required_attributes: string[]; banned_words: string[] },
) {
  return apiSend(`/v1/channels/${id}`, rulesResultSchema, {
    method: "PATCH",
    body: { config_hash: configHash, ...rules },
  });
}
