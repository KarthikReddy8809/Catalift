import { createMemoryHistory } from "@tanstack/react-router";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { beforeEach, describe, expect, it } from "vitest";

import { server } from "@/test/msw";
import { createTestQueryClient } from "@/test/render";

import { App } from "./App";

// A small in-memory API, answering the way the Go handlers do, so each page is
// exercised end to end through its queries and mutations.

type Role = "seller" | "reviewer";

interface Fake {
  role: Role | null;
  csrfSeen: string[];
  approved: Set<string>;
  version: number;
  exported: boolean;
  blocked: boolean;
}

let fake: Fake;

const err = (status: number, code: string, message: string) =>
  HttpResponse.json({ error: { code, message, request_id: "req_test" } }, { status });

function session(role: Role) {
  return {
    user: { id: "1", email: `${role}@example.com`, role },
    csrf_token: "csrf-1",
    expires_at: "2026-10-11T00:00:00Z",
  };
}

function listing(id: string, channel: string, rule: "passing" | "failing") {
  return {
    id,
    product_id: "1",
    sku: "KU-101",
    channel,
    status: "generated",
    failure_reason: null,
    version: fake.version,
    title: `Navy Kurta ${channel}`,
    bullets: ["a", "b", "c", "d", "e"],
    description: "A navy kurta.",
    rule_status: rule,
    rule_failures:
      rule === "failing"
        ? [{ rule: "banned_word", field: "bullet_5", message: "Banned word." }]
        : [],
    approved: fake.approved.has(id),
    approved_at: fake.approved.has(id) ? "2026-10-04T10:00:00Z" : null,
    approved_by: fake.approved.has(id) ? "reviewer@example.com" : null,
    attributes: { colour: "navy", pattern: "solid", sleeve: null, neckline: null, fit: "unknown" },
    attributes_revision: 1,
    latest_regeneration: null,
  };
}

const page = (data: unknown[]) => ({ data, page: { next_cursor: null, has_more: false } });

const product = {
  id: "1",
  sku: "KU-101",
  brand: { id: "1", name: "Indigo Loom" },
  category: "kurta",
  price_minor: 149900,
  currency: "INR",
  image_count: 1,
  attributes: {
    detection_status: "done",
    revision: 1,
    colour: "navy",
    pattern: "solid",
    sleeve: "full sleeve",
    neckline: "round neck",
    fit: "regular",
    detection_error: null,
  },
  ai_cost_micro_usd: 9200,
  created_at: "2026-10-04T09:00:00Z",
};

const run = {
  id: "7",
  created_at: "2026-10-04T09:00:00Z",
  detection: { pending: 0, done: 1, failed: 0, stopped_budget: 0 },
  listings: { queued: 0, generated: 1, failed: 1, stopped_budget: 0 },
};

function api() {
  const write = (request: Request) => {
    fake.csrfSeen.push(request.headers.get("X-CSRF-Token") ?? "");
  };
  return [
    http.get("*/v1/sessions/current", () =>
      fake.role ? HttpResponse.json(session(fake.role)) : err(401, "unauthorized", "Sign in."),
    ),
    http.post("*/v1/sessions", async ({ request }) => {
      const body = (await request.json()) as { email: string; password: string };
      if (body.password === "too-many") {
        return HttpResponse.json(
          { error: { code: "rate_limited", message: "Wait.", request_id: "r" } },
          { status: 429, headers: { "Retry-After": "42" } },
        );
      }
      if (body.password !== "right") return err(401, "unauthorized", "Wrong.");
      fake.role = body.email.startsWith("reviewer") ? "reviewer" : "seller";
      return HttpResponse.json(session(fake.role));
    }),
    http.delete("*/v1/sessions/current", ({ request }) => {
      write(request);
      fake.role = null;
      return new HttpResponse(null, { status: 204 });
    }),
    http.get("*/v1/budget", () =>
      HttpResponse.json({
        limit_micro_usd: 8_000_000,
        spent_micro_usd: 9200,
        blocked_at: fake.blocked ? "2026-10-04T10:00:00Z" : null,
      }),
    ),
    http.get("*/v1/brands", () =>
      HttpResponse.json(page([{ id: "1", name: "Indigo Loom", voice_note: null }])),
    ),
    http.get("*/v1/products", () => HttpResponse.json(page([product]))),
    http.get("*/v1/generation-runs/7", () => HttpResponse.json(run)),
    http.post("*/v1/generation-runs", ({ request }) => {
      write(request);
      return HttpResponse.json(run, { status: 201 });
    }),
    http.post("*/v1/generation-runs/7/resume", ({ request }) => {
      write(request);
      return HttpResponse.json(run);
    }),
    http.post("*/v1/uploads", ({ request }) => {
      write(request);
      return HttpResponse.json(
        {
          id: "3",
          file_name: "launch.csv",
          rows_total: 2,
          rows_accepted: 1,
          rows_rejected: 1,
          row_errors: [{ row_number: 3, sku: "KU-101", reason: "SKU already exists" }],
          ai_cost_micro_usd: 0,
          created_at: "2026-10-04T09:00:00Z",
        },
        { status: 201 },
      );
    }),
    http.post("*/v1/uploads/3/images", ({ request }) => {
      write(request);
      return HttpResponse.json(
        {
          attached: [{ file_name: "KU-101_front.png", sku: "KU-101", position: 1 }],
          unmatched_files: ["banner.png"],
          rejected_files: [{ file_name: "KU-101.gif", reason: "not a JPEG, PNG or WebP image" }],
          products_missing_image: ["KU-102"],
        },
        { status: 201 },
      );
    }),
    http.get("*/v1/listings", () =>
      HttpResponse.json(
        page([listing("11", "amazon_style", "passing"), listing("12", "own_website", "failing")]),
      ),
    ),
    http.patch("*/v1/listings/11", async ({ request }) => {
      write(request);
      const body = (await request.json()) as { version: number };
      if (body.version !== fake.version) return err(409, "version_conflict", "Changed.");
      fake.version += 1;
      return HttpResponse.json(listing("11", "amazon_style", "passing"));
    }),
    http.post("*/v1/listings/11/regeneration-requests", ({ request }) => {
      write(request);
      return HttpResponse.json(
        {
          id: "5",
          listing_id: "11",
          field: "title",
          instruction: "shorter",
          status: "queued",
          created_at: "2026-10-04T10:00:00Z",
        },
        { status: 201 },
      );
    }),
    http.post("*/v1/approvals", async ({ request }) => {
      write(request);
      const body = (await request.json()) as { items: { listing_id: string }[] };
      for (const i of body.items) fake.approved.add(i.listing_id);
      return HttpResponse.json(
        {
          approved: body.items.map((i) => ({
            listing_id: i.listing_id,
            version: fake.version,
            approved_at: "2026-10-04T10:00:00Z",
          })),
          skipped: [{ listing_id: "12", reason: "failing_rules" }],
        },
        { status: 201 },
      );
    }),
    http.get("*/v1/channels", () =>
      HttpResponse.json(
        page([
          {
            id: "amazon_style",
            name: "Amazon-style",
            enabled: true,
            load_error: null,
            title_max_length: 200,
            required_attributes: ["colour"],
            banned_words: ["sale"],
            export_headers: ["sku", "item_name"],
            last_recheck: {
              config_hash: "ab12",
              listings_rechecked: 3,
              approvals_cleared: 1,
              created_at: "2026-10-04T07:00:00Z",
            },
            listings_total: 2,
            listings_approved: fake.approved.size,
          },
          {
            id: "broken.yaml",
            name: "broken.yaml",
            enabled: false,
            load_error: "title_max_length must be positive",
            title_max_length: null,
            required_attributes: [],
            banned_words: [],
            export_headers: [],
            last_recheck: null,
            listings_total: 0,
            listings_approved: 0,
          },
        ]),
      ),
    ),
    http.post("*/v1/exports", ({ request }) => {
      write(request);
      if (fake.approved.size === 0) return err(422, "no_approved_listings", "Nothing.");
      fake.exported = true;
      return HttpResponse.json(
        {
          id: "9",
          created_at: "2026-10-04T11:00:00Z",
          files: [
            {
              channel: "amazon_style",
              row_count: 1,
              download_url: "/v1/exports/9/files/amazon_style",
            },
          ],
          skipped_channels: [],
        },
        { status: 201 },
      );
    }),
  ];
}

function renderAt(path: string) {
  return render(
    <App
      queryClient={createTestQueryClient()}
      history={createMemoryHistory({ initialEntries: [path] })}
    />,
  );
}

beforeEach(() => {
  fake = {
    role: "reviewer",
    csrfSeen: [],
    approved: new Set(),
    version: 2,
    exported: false,
    blocked: false,
  };
  server.use(...api());
  localStorage.clear();
});

describe("sign-in", () => {
  it("signs in and lands on products", async () => {
    fake.role = null;
    const user = userEvent.setup();
    renderAt("/sign-in");

    await user.type(await screen.findByLabelText("Email"), "seller@example.com");
    await user.type(screen.getByLabelText("Password"), "right");
    await user.click(screen.getByRole("button", { name: "Sign in" }));

    expect(await screen.findByRole("heading", { name: "Products" })).toBeInTheDocument();
    expect(screen.getByText("seller@example.com")).toBeInTheDocument();
  });

  it("says when the password is wrong", async () => {
    fake.role = null;
    const user = userEvent.setup();
    renderAt("/sign-in");

    await user.type(await screen.findByLabelText("Email"), "seller@example.com");
    await user.type(screen.getByLabelText("Password"), "wrong");
    await user.click(screen.getByRole("button", { name: "Sign in" }));

    expect(await screen.findByText("Email or password is wrong")).toBeInTheDocument();
  });

  it("shows the wait after too many attempts", async () => {
    fake.role = null;
    const user = userEvent.setup();
    renderAt("/sign-in?expired=true");
    expect(await screen.findByText("Your session ended")).toBeInTheDocument();

    await user.type(screen.getByLabelText("Email"), "seller@example.com");
    await user.type(screen.getByLabelText("Password"), "too-many");
    await user.click(screen.getByRole("button", { name: "Sign in" }));

    expect(await screen.findByText(/paused for 42 seconds/)).toBeInTheDocument();
  });

  it("sends a page visit with no session to sign-in", async () => {
    fake.role = null;

    renderAt("/review");

    expect(await screen.findByRole("button", { name: "Sign in" })).toBeInTheDocument();
  });

  it("signs out from the frame", async () => {
    const user = userEvent.setup();
    renderAt("/channels");

    await user.click(await screen.findByRole("button", { name: "Sign out" }));

    expect(await screen.findByRole("button", { name: "Sign in" })).toBeInTheDocument();
    expect(fake.csrfSeen).toContain("csrf-1");
  });
});

describe("upload", () => {
  it("uploads the CSV, then the photos, and reports each", async () => {
    const user = userEvent.setup();
    renderAt("/upload");

    await user.upload(
      await screen.findByLabelText("CSV file, up to 5 MB"),
      new File(["sku,category,brand,price\n"], "launch.csv", { type: "text/csv" }),
    );
    await user.click(screen.getByRole("button", { name: "Upload product list" }));
    expect(await screen.findByText("SKU already exists")).toBeInTheDocument();

    await user.upload(
      screen.getByLabelText("JPEG, PNG or WebP, up to 10 MB each"),
      new File(["png"], "KU-101_front.png", { type: "image/png" }),
    );
    await user.click(screen.getByRole("button", { name: "Upload photos" }));

    expect(await screen.findByText("banner.png")).toBeInTheDocument();
    expect(screen.getByText(/KU-101.gif/)).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Go to products" }));
    expect(await screen.findByRole("heading", { name: "Products" })).toBeInTheDocument();
    expect(fake.csrfSeen).toEqual(["csrf-1", "csrf-1"]);
  });

  it("names a file that is too large", async () => {
    server.use(http.post("*/v1/uploads", () => err(413, "payload_too_large", "Too big.")));
    const user = userEvent.setup();
    renderAt("/upload");

    await user.upload(
      await screen.findByLabelText("CSV file, up to 5 MB"),
      new File(["x"], "huge.csv", { type: "text/csv" }),
    );
    await user.click(screen.getByRole("button", { name: "Upload product list" }));
    expect(await screen.findByText("huge.csv is too large")).toBeInTheDocument();
  });
});

describe("products", () => {
  it("asks for a neutral voice, starts a run and shows its progress", async () => {
    const user = userEvent.setup();
    renderAt("/products?upload=3");

    expect(await screen.findAllByText("KU-101")).not.toHaveLength(0);
    await user.click(screen.getByRole("checkbox", { name: /neutral voice/ }));
    const alert = screen.getByRole("alert");
    await user.click(within(alert).getByRole("button", { name: "Generate listings" }));

    expect(await screen.findByText(/written/)).toBeInTheDocument();
    await user.click(await screen.findByRole("button", { name: /Resume 1 failed/ }));
    await waitFor(() => {
      expect(fake.csrfSeen).toHaveLength(2);
    });
  });

  it("shows the blocked budget banner", async () => {
    fake.blocked = true;

    renderAt("/products");

    expect(await screen.findByText("AI calls are blocked")).toBeInTheDocument();
  });
});

describe("review", () => {
  it("opens a listing, saves an edit and asks for a rewrite", async () => {
    const user = userEvent.setup();
    renderAt("/review");

    const links = await screen.findAllByRole("button", { name: "Navy Kurta amazon_style" });
    await user.click(links[0]!);
    const title = await screen.findByLabelText("Title");
    await user.clear(title);
    await user.type(title, "Navy Cotton Kurta");
    await user.click(screen.getByRole("button", { name: "Save changes" }));
    await waitFor(() => {
      expect(fake.version).toBe(3);
    });

    await user.type(screen.getByPlaceholderText(/shorter/), "shorter");
    await user.click(screen.getByRole("button", { name: "Rewrite title" }));
    await waitFor(() => {
      expect(fake.csrfSeen).toHaveLength(2);
    });
    await user.click(screen.getByRole("button", { name: "Close" }));
    expect(screen.queryByLabelText("Title")).not.toBeInTheDocument();
  });

  it("explains a conflicting edit", async () => {
    server.use(http.patch("*/v1/listings/11", () => err(409, "version_conflict", "Changed.")));
    const user = userEvent.setup();
    renderAt("/review");

    const links = await screen.findAllByRole("button", { name: "Navy Kurta amazon_style" });
    await user.click(links[0]!);
    await user.type(await screen.findByLabelText("Description"), " More.");
    await user.click(screen.getByRole("button", { name: "Save changes" }));

    expect(
      await screen.findByText(/changed this listing while you were editing/),
    ).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Reload the listing" }));
  });

  it("approves the selected listings and reports the skipped", async () => {
    const user = userEvent.setup();
    renderAt("/review");

    const boxes = await screen.findAllByRole("checkbox", { name: /Select KU-101 on Amazon-style/ });
    await user.click(boxes[0]!);
    await user.click(screen.getByRole("tab", { name: "Not approved" }));
    await user.click(screen.getByRole("button", { name: "Approve 1 selected" }));

    expect(await screen.findByText("1 approved, 1 skipped")).toBeInTheDocument();
    expect(fake.approved.has("11")).toBe(true);
  });

  it("keeps a seller read-only", async () => {
    fake.role = "seller";

    renderAt("/review");

    expect(await screen.findAllByText("Not approved")).not.toHaveLength(0);
    expect(screen.queryByRole("button", { name: /Approve/ })).not.toBeInTheDocument();
  });
});

describe("export and channels", () => {
  it("exports the approved listings and offers the download", async () => {
    fake.approved.add("11");
    const user = userEvent.setup();
    renderAt("/export");

    await user.click(await screen.findByRole("button", { name: "Export 1 approved listings" }));

    expect(await screen.findByRole("link", { name: /Download Amazon-style CSV/ })).toHaveAttribute(
      "href",
      expect.stringContaining("/v1/exports/9/files/amazon_style"),
    );
  });

  it("tells a seller that export is for reviewers", async () => {
    fake.role = "seller";

    renderAt("/export");

    expect(await screen.findByText("Export is for reviewers")).toBeInTheDocument();
  });

  it("lists the channels, including one switched off", async () => {
    renderAt("/channels");

    expect(await screen.findByText(/title_max_length must be positive/)).toBeInTheDocument();
  });

  it("navigates between pages from the frame", async () => {
    const user = userEvent.setup();
    renderAt("/channels");

    const nav = await screen.findByRole("navigation", { name: "Catalift" });
    await user.click(within(nav).getByRole("link", { name: "Upload" }));

    expect(await screen.findByRole("heading", { name: "Upload a launch" })).toBeInTheDocument();
  });
});
