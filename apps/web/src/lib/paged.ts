import { z } from "zod";

import { apiFetch } from "@/lib/api";

const pageSchema = z.object({
  next_cursor: z.string().nullable().optional(),
  has_more: z.boolean(),
});

/**
 * fetchAllPages follows the cursor until the last page. The demo holds at
 * most a few hundred rows per list (PRD scale), so one screen shows them all.
 */
export async function fetchAllPages<T>(
  path: string,
  item: z.ZodType<T>,
  signal?: AbortSignal,
  maxPages = 20,
): Promise<T[]> {
  const schema = z.object({ data: z.array(item), page: pageSchema });
  const out: T[] = [];
  let cursor: string | null | undefined = null;
  for (let i = 0; i < maxPages; i++) {
    const sep = path.includes("?") ? "&" : "?";
    const url = `${path}${sep}limit=100${cursor ? `&cursor=${encodeURIComponent(cursor)}` : ""}`;
    const res: z.infer<typeof schema> = await apiFetch(url, schema, signal ? { signal } : {});
    out.push(...res.data);
    if (!res.page.has_more) break;
    cursor = res.page.next_cursor;
  }
  return out;
}
