import type { Product } from "./api";

/**
 * newestFirst puts the latest upload at the top and keeps each upload in the
 * order its CSV listed the products (ids grow in load order). Without an
 * upload id (an older API), products of one upload share created_at, since
 * an upload writes them in one transaction.
 */
export function newestFirst(a: Product, b: Product): number {
  if (a.upload_id && b.upload_id && a.upload_id !== b.upload_id) {
    return Number(b.upload_id) - Number(a.upload_id);
  }
  if (!a.upload_id && a.created_at !== b.created_at) return a.created_at < b.created_at ? 1 : -1;
  return Number(a.id) - Number(b.id);
}
