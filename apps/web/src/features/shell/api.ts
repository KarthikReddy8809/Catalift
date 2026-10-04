import { queryOptions } from "@tanstack/react-query";
import { z } from "zod";

import { apiFetch } from "@/lib/api";

export const budgetSchema = z.object({
  limit_micro_usd: z.number(),
  spent_micro_usd: z.number(),
  blocked_at: z.string().nullable(),
});

export type Budget = z.infer<typeof budgetSchema>;

/** budgetQueryOptions polls the spend so the header follows a running batch. */
export function budgetQueryOptions() {
  return queryOptions({
    queryKey: ["budget"] as const,
    queryFn: ({ signal }) => apiFetch("/v1/budget", budgetSchema, { signal }),
    refetchInterval: 15_000,
  });
}
