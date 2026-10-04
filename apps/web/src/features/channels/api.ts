import { queryOptions } from "@tanstack/react-query";
import { z } from "zod";

import { fetchAllPages } from "@/lib/paged";

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
  listings_total: z.number(),
  listings_approved: z.number(),
});

export type Channel = z.infer<typeof channelSchema>;

export function channelsQueryOptions() {
  return queryOptions({
    queryKey: ["channels"] as const,
    queryFn: ({ signal }) => fetchAllPages("/v1/channels", channelSchema, signal),
  });
}
