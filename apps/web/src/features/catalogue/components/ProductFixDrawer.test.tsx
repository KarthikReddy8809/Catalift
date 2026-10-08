import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { ProductFixDrawer, type FixTarget } from "./ProductFixDrawer";

const target: FixTarget = {
  key: "product:1",
  title: "KU-101",
  problem: "The AI could not finish this product.",
  values: { sku: "KU-101", category: "kurta", brand: "Indigo Loom", price: "1299" },
  photoCount: 2,
};

describe("ProductFixDrawer", () => {
  it("says how many photos are on file and that Validate runs the AI", () => {
    render(<ProductFixDrawer target={target} categories={["kurta"]} onClose={vi.fn()} />);

    expect(screen.getByText("2 photos on file.")).toBeInTheDocument();
    expect(screen.getByText(/runs the AI on this product again/)).toBeInTheDocument();
  });

  it("names one photo in the singular", () => {
    render(
      <ProductFixDrawer
        target={{ ...target, photoCount: 1 }}
        categories={["kurta"]}
        onClose={vi.fn()}
      />,
    );

    expect(screen.getByText("1 photo on file.")).toBeInTheDocument();
  });

  it("shows a success outcome as a status, not an alert", () => {
    render(
      <ProductFixDrawer
        target={target}
        categories={["kurta"]}
        outcome={{ tone: "success", message: "Saved." }}
        onClose={vi.fn()}
      />,
    );

    expect(screen.getByRole("status")).toHaveTextContent("FixedSaved.");
  });

  it("disables Validate and Discard while a validation runs", () => {
    render(
      <ProductFixDrawer
        target={target}
        categories={["kurta"]}
        busy
        onDiscard={vi.fn()}
        onClose={vi.fn()}
      />,
    );

    expect(screen.getByRole("button", { name: "Validating…" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Discard this row" })).toBeDisabled();
  });

  it("keeps a category the file used that is not on the list", () => {
    render(
      <ProductFixDrawer
        target={{ ...target, values: { ...target.values, category: "jeans" } }}
        categories={["kurta"]}
        onClose={vi.fn()}
      />,
    );

    expect(screen.getByRole("combobox", { name: "Category" })).toHaveTextContent("jeans");
  });

  it("drops files that are not images and closes on Escape", async () => {
    const onClose = vi.fn();
    const user = userEvent.setup({ applyAccept: false });
    render(<ProductFixDrawer target={target} categories={["kurta"]} onClose={onClose} />);

    await user.upload(screen.getByLabelText("Photos for this product"), [
      new File(["a"], "front.png", { type: "image/png" }),
      new File(["b"], "notes.txt", { type: "text/plain" }),
    ]);
    await user.keyboard("{Escape}");

    expect(screen.queryByText(/notes\.txt/)).not.toBeInTheDocument();
    expect(onClose).toHaveBeenCalledOnce();
  });
});
