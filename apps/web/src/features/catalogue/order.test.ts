import { describe, expect, it } from "vitest";

import type { Product } from "./api";
import { newestFirst } from "./order";

const at = (id: string, sku: string, createdAt: string) =>
  ({ id, sku, created_at: createdAt }) as Product;

describe("newestFirst", () => {
  it("puts the latest upload first and keeps each upload in CSV order", () => {
    const older = [
      at("1", "AA-1", "2026-10-04T08:00:00Z"),
      at("2", "ZZ-9", "2026-10-04T08:00:00Z"),
    ];
    const latest = [
      at("7", "MM-2", "2026-10-07T09:00:00Z"),
      at("8", "BB-5", "2026-10-07T09:00:00Z"),
    ];

    const sorted = [...older, ...latest].sort(newestFirst).map((p) => p.sku);

    expect(sorted).toEqual(["MM-2", "BB-5", "AA-1", "ZZ-9"]);
  });
});

describe("newestFirst with upload ids", () => {
  it("orders by upload, newest first, even for a product fixed later", () => {
    const fixedLater = {
      id: "50",
      sku: "OLD-1",
      upload_id: "3",
      created_at: "2026-10-08T09:00:00Z",
    } as Product;
    const fresh = {
      id: "40",
      sku: "NEW-1",
      upload_id: "12",
      created_at: "2026-10-07T09:00:00Z",
    } as Product;

    const sorted = [fixedLater, fresh].sort(newestFirst).map((p) => p.sku);

    expect(sorted).toEqual(["NEW-1", "OLD-1"]);
  });
});
