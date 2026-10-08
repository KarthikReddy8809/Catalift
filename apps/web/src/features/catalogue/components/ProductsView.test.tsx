import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";

import { PAGE_SIZE, ProductsView, type ProductRow } from "./ProductsView";

const product = (n: number): ProductRow => ({
  id: String(n),
  sku: `KU-${String(100 + n)}`,
  brand: "Indigo Loom",
  category: "kurta",
  priceMinor: 129_900,
  imageCount: 1,
  detection: "done",
  attributes: [
    { name: "Colour", value: "navy" },
    { name: "Pattern", value: "solid" },
  ],
  aiCostMicroUsd: 1_000,
});

const many = Array.from({ length: PAGE_SIZE + 3 }, (_, i) => product(i + 1));

describe("ProductsView table", () => {
  it("shows one page of rows and moves to the next", async () => {
    const user = userEvent.setup();
    render(<ProductsView status="ready" products={many} />);

    expect(screen.getByText("Showing 1 to 10 of 13")).toBeInTheDocument();
    expect(screen.queryByText("KU-111")).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Next" }));

    expect(screen.getByText("KU-111")).toBeInTheDocument();
    expect(screen.getByText("Showing 11 to 13 of 13")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Next" })).toBeDisabled();
  });

  it("goes back a page", async () => {
    const user = userEvent.setup();
    render(<ProductsView status="ready" products={many} />);

    await user.click(screen.getByRole("button", { name: "Next" }));
    await user.click(screen.getByRole("button", { name: "Previous" }));

    expect(screen.getByText("Page 1 of 2")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Previous" })).toBeDisabled();
  });

  it("finds a product by its SKU, ignoring case", async () => {
    const user = userEvent.setup();
    render(<ProductsView status="ready" products={many} />);

    await user.type(screen.getByLabelText("Search products by SKU or product ID"), "ku-112");

    expect(screen.getByText("KU-112")).toBeInTheDocument();
    expect(screen.getByText("Showing 1 to 1 of 1")).toBeInTheDocument();
  });

  it("finds a product by its id", async () => {
    const user = userEvent.setup();
    render(<ProductsView status="ready" products={many} />);

    await user.type(screen.getByLabelText("Search products by SKU or product ID"), "13");

    expect(screen.getByText("KU-113")).toBeInTheDocument();
    expect(screen.getByText("Showing 1 to 1 of 1")).toBeInTheDocument();
  });

  it("says when nothing matches the search", async () => {
    const user = userEvent.setup();
    render(<ProductsView status="ready" products={many} />);

    await user.type(screen.getByLabelText("Search products by SKU or product ID"), "SA-9");

    expect(screen.getByText("No product matches “SA-9”.")).toBeInTheDocument();
    expect(screen.getByText("No rows")).toBeInTheDocument();
  });

  it("finds a rejected row by its SKU", async () => {
    const user = userEvent.setup();
    render(
      <ProductsView
        status="ready"
        products={many}
        rejected={[
          {
            uploadId: "3",
            fileName: "launch.csv",
            rowNumber: 4,
            sku: "SA-201",
            category: "jeans",
            brand: "Indigo Loom",
            price: "abc",
            reason: "SKU already exists",
          },
        ]}
      />,
    );

    await user.type(screen.getByLabelText("Search products by SKU or product ID"), "sa-201");

    expect(screen.getByText("SKU already exists")).toBeInTheDocument();
  });

  it("shows each attribute with its name", () => {
    render(<ProductsView status="ready" products={[product(1)]} />);

    const list = screen.getByRole("list", { name: "Attributes of KU-101" });
    expect(within(list).getByText("Colour")).toBeInTheDocument();
    expect(within(list).getByText("navy")).toBeInTheDocument();
    expect(within(list).getByText("Pattern")).toBeInTheDocument();
  });

  it("says when a product's attributes are not read yet", () => {
    render(
      <ProductsView
        status="ready"
        products={[{ ...product(1), detection: "pending", attributes: [] }]}
      />,
    );

    expect(screen.getByText("Not read yet")).toBeInTheDocument();
  });
});

describe("ProductsView order", () => {
  // Regression: refused rows from earlier uploads were always listed first,
  // so more than a page of them pushed a new upload's products off page 1.
  it("lists the newest upload first, ahead of older refused rows", () => {
    const oldRejected = Array.from({ length: PAGE_SIZE + 2 }, (_, i) => ({
      uploadId: "3",
      fileName: "products.csv",
      rowNumber: i + 2,
      sku: `KK-${String(5001 + i)}`,
      category: "kurta",
      brand: "",
      price: "",
      reason: "SKU already exists",
    }));
    const fresh = [
      { ...product(1), sku: "DV-4101", uploadId: "12" },
      { ...product(2), sku: "DV-4102", uploadId: "12" },
    ];

    render(<ProductsView status="ready" products={fresh} rejected={oldRejected} />);

    const rows = screen.getAllByRole("row").slice(1, 4);
    expect(rows.map((r) => r.querySelector("td")?.textContent)).toEqual([
      "DV-4101",
      "DV-4102",
      "KK-5001",
    ]);
  });

  it("keeps an upload's own refused rows just above its products", () => {
    const rejectedNow = {
      uploadId: "12",
      fileName: "products.csv",
      rowNumber: 14,
      sku: "KV-2207",
      category: "shirt",
      brand: "Kesar Vann",
      price: "1099",
      reason: "category is not one Catalift handles",
    };
    const older = { ...product(1), sku: "AA-100", uploadId: "3" };
    const fresh = { ...product(9), sku: "KV-2201", uploadId: "12" };

    render(<ProductsView status="ready" products={[older, fresh]} rejected={[rejectedNow]} />);

    const rows = screen.getAllByRole("row").slice(1, 4);
    expect(rows.map((r) => r.querySelector("td")?.textContent)).toEqual([
      "KV-2207",
      "KV-2201",
      "AA-100",
    ]);
  });
});
