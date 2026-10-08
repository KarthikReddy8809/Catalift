import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { allScreens } from "./registry";

// Every state of every screen design renders without throwing and shows a
// heading, so a design that breaks is caught before anyone opens the gallery.
describe("screen designs", () => {
  const cases = allScreens().flatMap((s) =>
    Object.entries(s.states).map(([state, renderState]) => ({ id: s.id, state, renderState })),
  );

  it.each(cases)("$id renders its $state state", ({ renderState }) => {
    const { container } = render(<>{renderState()}</>);
    expect(container).not.toBeEmptyDOMElement();
    expect(
      screen.queryAllByRole("heading").length + screen.queryAllByRole("status").length,
    ).toBeGreaterThan(0);
  });

  it("finds every designed screen", () => {
    expect(allScreens().map((s) => s.id)).toEqual([
      "S-01",
      "S-02",
      "S-03",
      "S-04",
      "S-05",
      "S-06",
      "S-07",
      "S-08",
      "S-09",
    ]);
  });
});
