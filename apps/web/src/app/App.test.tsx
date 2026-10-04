import { createMemoryHistory } from "@tanstack/react-router";
import { render, screen } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { describe, expect, it } from "vitest";

import { server } from "@/test/msw";
import { createTestQueryClient } from "@/test/render";

import { App } from "./App";

function renderAt(path: string) {
  return render(
    <App
      queryClient={createTestQueryClient()}
      history={createMemoryHistory({ initialEntries: [path] })}
    />,
  );
}

describe("App", () => {
  it("sends a visitor with no session from / to sign-in", async () => {
    server.use(
      http.get("*/v1/sessions/current", () =>
        HttpResponse.json(
          { error: { code: "unauthorized", message: "Sign in to continue.", request_id: "r1" } },
          { status: 401 },
        ),
      ),
    );

    renderAt("/");

    expect(await screen.findByRole("button", { name: "Sign in" })).toBeInTheDocument();
    expect(screen.getByRole("navigation", { name: "Main" })).toBeInTheDocument();
  });

  it("renders the not-found route for an unknown path", async () => {
    renderAt("/nowhere");

    expect(await screen.findByRole("heading", { name: "Page not found" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Back to the start" })).toHaveAttribute("href", "/");
  });
});
