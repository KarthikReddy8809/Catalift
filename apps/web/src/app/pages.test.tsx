import { createMemoryHistory } from "@tanstack/react-router";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
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
  bannedSaved: string[] | null;
  rowFixed: boolean;
  photoAdded: string | null;
  productSaved: string | null;
  enriched: string | null;
  sent: boolean;
  brandSaved: { voice_note: string | null; words_to_avoid: string[] } | null;
}

let fake: Fake;

const exportNine = {
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
};

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
  status: "ready_for_review",
  listings: { total: 2, failing_rules: 1, approved: 0 },
  attributes: {
    detection_status: "done",
    confidence: 0.42,
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
      HttpResponse.json(
        page([
          {
            id: "1",
            name: "Indigo Loom",
            voice_note: fake.brandSaved?.voice_note ?? null,
            words_to_avoid: fake.brandSaved?.words_to_avoid ?? [],
            updated_at: fake.brandSaved ? "2026-10-07T10:00:00Z" : "2026-10-04T08:00:00Z",
          },
        ]),
      ),
    ),
    http.patch("*/v1/brands/1", async ({ request }) => {
      write(request);
      if (fake.role !== "seller") return err(403, "forbidden_role", "Sellers set brand voice.");
      const body = (await request.json()) as {
        voice_note: string | null;
        words_to_avoid: string[];
      };
      fake.brandSaved = body;
      return HttpResponse.json({
        id: "1",
        name: "Indigo Loom",
        ...body,
        updated_at: "2026-10-07T10:00:00Z",
      });
    }),
    http.get("*/v1/exports", () => {
      const sentExport = {
        ...exportNine,
        sent_at: fake.sent ? "2026-10-04T11:05:00Z" : null,
        sent_by: fake.sent ? "reviewer@example.com" : null,
      };
      const visible = fake.role === "seller" ? fake.sent : fake.exported;
      return HttpResponse.json(page(visible ? [sentExport] : []));
    }),
    http.post("*/v1/exports/9/send", ({ request }) => {
      write(request);
      fake.sent = true;
      return HttpResponse.json({
        ...exportNine,
        sent_at: "2026-10-04T11:05:00Z",
        sent_by: "reviewer@example.com",
      });
    }),
    http.get("*/v1/products", () => HttpResponse.json(page([product]))),
    http.get("*/v1/row-errors", () =>
      HttpResponse.json({
        data: fake.rowFixed
          ? []
          : [
              {
                upload_id: "3",
                file_name: "launch.csv",
                row_number: 4,
                sku: "KU-103",
                reason: 'category "jeans" is not one Catalift handles; use one of: kurta, saree',
                category: "jeans",
                brand: "Indigo Loom",
                price: "abc",
              },
            ],
        categories: ["kurta", "saree"],
      }),
    ),
    http.delete("*/v1/uploads/3/rows/4", ({ request }) => {
      write(request);
      fake.rowFixed = true;
      return new HttpResponse(null, { status: 204 });
    }),
    http.delete("*/v1/row-errors", ({ request }) => {
      write(request);
      fake.rowFixed = true;
      return HttpResponse.json({ discarded: 1 });
    }),
    http.put("*/v1/uploads/3/rows/4", async ({ request }) => {
      write(request);
      const body = (await request.json()) as { category: string; price: string };
      if (body.category !== "kurta" || body.price !== "1299") {
        return HttpResponse.json({
          status: "rejected",
          reason: "price must be a positive number of rupees",
          product_id: null,
        });
      }
      fake.rowFixed = true;
      return HttpResponse.json({ status: "loaded", reason: null, product_id: "2" });
    }),
    http.post("*/v1/products/:id/images", ({ request, params }) => {
      write(request);
      fake.photoAdded = String(params.id);
      return HttpResponse.json(
        {
          attached: [{ file_name: "front.png", sku: "KU-101", position: 1 }],
          unmatched_files: [],
          rejected_files: [],
          products_missing_image: [],
        },
        { status: 201 },
      );
    }),
    http.patch("*/v1/products/:id", async ({ request }) => {
      write(request);
      const body = (await request.json()) as { price: string };
      fake.productSaved = body.price;
      return HttpResponse.json(
        body.price === "abc"
          ? { status: "rejected", reason: "price must be a positive number of rupees" }
          : { status: "saved", reason: null },
      );
    }),
    http.post("*/v1/products/:id/enrich", ({ request, params }) => {
      write(request);
      fake.enriched = String(params.id);
      return HttpResponse.json(run, { status: 201 });
    }),
    http.get("*/v1/generation-runs/7", () => HttpResponse.json(run)),
    http.post("*/v1/generation-runs", ({ request }) => {
      write(request);
      return HttpResponse.json(run, { status: 201 });
    }),
    http.post("*/v1/generation-runs/7/resume", ({ request }) => {
      write(request);
      return HttpResponse.json(run);
    }),
    http.get("*/v1/uploads/latest", () =>
      HttpResponse.json({
        id: "3",
        file_name: "launch.csv",
        rows_total: 2,
        rows_accepted: 1,
        rows_rejected: 1,
        row_errors: [],
        ai_cost_micro_usd: 0,
        created_at: "2026-10-04T09:00:00Z",
      }),
    ),
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
    http.post("*/v1/rule-checks", async ({ request }) => {
      write(request);
      const body = (await request.json()) as { title?: string };
      const bad = body.title?.toLowerCase().includes("guaranteed") ?? false;
      return HttpResponse.json({
        rule_status: bad ? "failing" : "passing",
        rule_failures: bad
          ? [{ rule: "banned_word", field: "title", message: 'Title uses "guaranteed".' }]
          : [],
      });
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
            config_hash: "hash-1",
            last_edit: { by: "reviewer@example.com", at: "2026-10-05T06:00:00Z" },
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
            config_hash: null,
            last_edit: null,
            listings_total: 0,
            listings_approved: 0,
          },
        ]),
      ),
    ),
    http.patch("*/v1/channels/amazon_style", async ({ request }) => {
      write(request);
      const body = (await request.json()) as { config_hash: string; banned_words: string[] };
      if (body.config_hash !== "hash-1") return err(409, "version_conflict", "Changed.");
      fake.bannedSaved = body.banned_words;
      return HttpResponse.json({
        channel: "amazon_style",
        listings_rechecked: 2,
        approvals_cleared: 1,
      });
    }),
    http.post("*/v1/exports", ({ request }) => {
      write(request);
      if (fake.approved.size === 0) return err(422, "no_approved_listings", "Nothing.");
      fake.exported = true;
      return HttpResponse.json({ ...exportNine, sent_at: null, sent_by: null }, { status: 201 });
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
    bannedSaved: null,
    rowFixed: false,
    photoAdded: null,
    productSaved: null,
    enriched: null,
    sent: false,
    brandSaved: null,
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
    fake.role = "seller";
    const user = userEvent.setup();
    renderAt("/upload");

    await user.upload(
      await screen.findByLabelText("CSV file, up to 5 MB"),
      new File(["sku,category,brand,price\n"], "launch.csv", { type: "text/csv" }),
    );
    await user.click(screen.getByRole("button", { name: "Upload product list" }));
    expect(await screen.findByText("SKU already exists")).toBeInTheDocument();

    await user.upload(screen.getByLabelText("Photos folder"), [
      new File(["png"], "KU-101_front.png", { type: "image/png" }),
      new File(["x"], ".DS_Store"),
    ]);
    expect(screen.getByText(/other files left out/)).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Upload 1 photos" }));

    expect(await screen.findByText("banner.png")).toBeInTheDocument();
    expect(screen.getByRole("cell", { name: "KU-101_front.png" })).toBeInTheDocument();
    expect(screen.getByText(/KU-101.gif/)).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Go to products" }));
    expect(await screen.findByRole("heading", { name: "Products" })).toBeInTheDocument();
    expect(fake.csrfSeen).toEqual(["csrf-1", "csrf-1"]);
  });

  it("takes photos dropped on the picker and can clear them", async () => {
    fake.role = "seller";
    const user = userEvent.setup();
    renderAt("/upload");
    await user.upload(
      await screen.findByLabelText("CSV file, up to 5 MB"),
      new File(["sku,category,brand,price\n"], "launch.csv", { type: "text/csv" }),
    );
    await user.click(screen.getByRole("button", { name: "Upload product list" }));
    const zone = await screen.findByRole("group", { name: "Drop photos here" });

    fireEvent.dragOver(zone);
    fireEvent.drop(zone, {
      dataTransfer: {
        items: [],
        files: [
          new File(["a"], "KU-101_front.jpg", { type: "image/jpeg" }),
          new File(["b"], "KU-101_back.webp", { type: "image/webp" }),
        ],
      },
    });

    expect(await screen.findByRole("button", { name: "Upload 2 photos" })).toBeEnabled();
    await user.click(screen.getByRole("button", { name: "Clear the chosen photos" }));
    expect(screen.getByRole("button", { name: "Upload 0 photos" })).toBeDisabled();
  });

  it("names a file that is too large", async () => {
    fake.role = "seller";
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

  it("shows each product's status", async () => {
    renderAt("/products");

    expect((await screen.findAllByText("Ready for review")).length).toBeGreaterThan(0);
  });

  it("lists products from an API one release behind (no status yet)", async () => {
    const older: Record<string, unknown> = { ...product };
    delete older.status;
    delete older.listings;
    server.use(http.get("*/v1/products", () => HttpResponse.json(page([older]))));

    renderAt("/products");

    expect((await screen.findAllByText("KU-101")).length).toBeGreaterThan(0);
    expect(screen.getAllByText("Ready for review").length).toBeGreaterThan(0);
  });

  it("lists the newest upload, not one this browser remembers", async () => {
    localStorage.setItem(
      "catalift.launch",
      JSON.stringify({ uploadId: "999", fileName: "old.csv" }),
    );
    let asked = "";
    server.use(
      http.get("*/v1/products", ({ request }) => {
        asked = new URL(request.url).search;
        return HttpResponse.json(page([product]));
      }),
    );

    renderAt("/products");

    expect((await screen.findAllByText("KU-101")).length).toBeGreaterThan(0);
    expect(asked).toContain("filter[upload_id]=3");
    expect(asked).not.toContain("999");
    expect(screen.getAllByText(/launch\.csv/).length).toBeGreaterThan(0);
  });

  it("says why the products did not load", async () => {
    server.use(
      http.get("*/v1/products", () => err(500, "internal", "Something failed on our side.")),
    );

    renderAt("/products");

    expect(await screen.findByText(/Something failed on our side\./)).toBeInTheDocument();
    expect(screen.getByText(/req_test/)).toBeInTheDocument();
  });

  it("opens a rejected row in the drawer, loads it, adds its photo and runs the AI", async () => {
    fake.role = "seller";
    const user = userEvent.setup();
    renderAt("/products");

    await user.click(
      (await screen.findAllByRole("button", { name: "Fix row 4 of launch.csv" }))[0]!,
    );
    const price = await screen.findByLabelText("Price (₹)");
    await user.clear(price);
    await user.type(price, "1299");
    await user.click(screen.getByRole("combobox", { name: "Category" }));
    await user.click(await screen.findByRole("option", { name: "kurta" }));
    await user.upload(screen.getByLabelText("Photos for this product"), [
      new File(["png"], "front.png", { type: "image/png" }),
    ]);
    await user.click(screen.getByRole("button", { name: "Validate" }));

    await waitFor(() => {
      expect(fake.enriched).toBe("2");
    });
    expect(fake.rowFixed).toBe(true);
    expect(fake.photoAdded).toBe("2");
    await waitFor(() => {
      expect(screen.queryByRole("button", { name: "Validate" })).not.toBeInTheDocument();
    });
  });

  it("discards every rejected row at once", async () => {
    fake.role = "seller";
    const user = userEvent.setup();
    renderAt("/products");

    await user.click(await screen.findByRole("button", { name: "Discard all 1" }));

    await waitFor(() => {
      expect(screen.queryAllByText("Not loaded")).toHaveLength(0);
    });
    expect(fake.rowFixed).toBe(true);
  });

  it("discards one rejected row from the drawer", async () => {
    fake.role = "seller";
    const user = userEvent.setup();
    renderAt("/products");

    await user.click(
      (await screen.findAllByRole("button", { name: "Fix row 4 of launch.csv" }))[0]!,
    );
    await user.click(await screen.findByRole("button", { name: "Discard this row" }));

    await waitFor(() => {
      expect(screen.queryByRole("button", { name: "Validate" })).not.toBeInTheDocument();
    });
    expect(fake.rowFixed).toBe(true);
  });

  it("says to restart the API when it lacks the re-run route", async () => {
    fake.role = "seller";
    server.use(
      http.get("*/v1/products", () =>
        HttpResponse.json(page([{ ...product, image_count: 0, status: "needs_photo" }])),
      ),
      http.post(
        "*/v1/products/:id/enrich",
        () => new HttpResponse("404 page not found", { status: 404 }),
      ),
    );
    const user = userEvent.setup();
    renderAt("/products");

    await user.click((await screen.findAllByRole("button", { name: "Fix KU-101" }))[0]!);
    await user.upload(await screen.findByLabelText("Photos for this product"), [
      new File(["png"], "front.png", { type: "image/png" }),
    ]);
    await user.click(screen.getByRole("button", { name: "Validate" }));

    expect(await screen.findByText(/restart it \(make dev\)/)).toBeInTheDocument();
  });

  it("keeps a rejected row in the drawer when the file's values are still invalid", async () => {
    fake.role = "seller";
    const user = userEvent.setup();
    renderAt("/products");

    await user.click(
      (await screen.findAllByRole("button", { name: "Fix row 4 of launch.csv" }))[0]!,
    );
    await user.click(await screen.findByRole("button", { name: "Validate" }));

    expect(
      await screen.findByText("price must be a positive number of rupees"),
    ).toBeInTheDocument();
    expect(fake.rowFixed).toBe(false);
  });

  it("saves a product's details and asks for a photo before the AI runs", async () => {
    fake.role = "seller";
    server.use(
      http.get("*/v1/products", () =>
        HttpResponse.json(page([{ ...product, image_count: 0, status: "needs_photo" }])),
      ),
    );
    const user = userEvent.setup();
    renderAt("/products");

    await user.click((await screen.findAllByRole("button", { name: "Fix KU-101" }))[0]!);
    await user.click(await screen.findByRole("button", { name: "Validate" }));

    expect(
      await screen.findByText("Details saved. Add a photo so the AI can read the garment."),
    ).toBeInTheDocument();
    expect(fake.enriched).toBeNull();
  });

  it("says which photos were refused when none could be added", async () => {
    fake.role = "seller";
    server.use(
      http.get("*/v1/products", () =>
        HttpResponse.json(page([{ ...product, image_count: 0, status: "needs_photo" }])),
      ),
      http.post("*/v1/products/:id/images", () =>
        HttpResponse.json(
          {
            attached: [],
            unmatched_files: [],
            rejected_files: [{ file_name: "front.png", reason: "not a JPEG, PNG or WebP image" }],
            products_missing_image: [],
          },
          { status: 201 },
        ),
      ),
    );
    const user = userEvent.setup();
    renderAt("/products");

    await user.click((await screen.findAllByRole("button", { name: "Fix KU-101" }))[0]!);
    await user.upload(await screen.findByLabelText("Photos for this product"), [
      new File(["png"], "front.png", { type: "image/png" }),
    ]);
    await user.click(screen.getByRole("button", { name: "Validate" }));

    expect(
      await screen.findByText(
        "Details saved; photos not added: front.png (not a JPEG, PNG or WebP image).",
      ),
    ).toBeInTheDocument();
    expect(fake.enriched).toBeNull();
  });

  it("shows the server's reason and request id when the re-run is refused", async () => {
    fake.role = "seller";
    server.use(
      http.get("*/v1/products", () => HttpResponse.json(page([{ ...product, status: "failed" }]))),
      http.post("*/v1/products/:id/enrich", () =>
        err(402, "budget_exhausted", "The AI budget is spent."),
      ),
    );
    const user = userEvent.setup();
    renderAt("/products");

    await user.click((await screen.findAllByRole("button", { name: "Fix KU-101" }))[0]!);
    await user.click(await screen.findByRole("button", { name: "Validate" }));

    expect(
      await screen.findByText("The AI budget is spent. (request req_test)"),
    ).toBeInTheDocument();
  });

  it("keeps a record in the drawer with its reason when it is still invalid", async () => {
    fake.role = "seller";
    server.use(
      http.get("*/v1/products", () =>
        HttpResponse.json(page([{ ...product, image_count: 0, status: "needs_photo" }])),
      ),
    );
    const user = userEvent.setup();
    renderAt("/products");

    await user.click((await screen.findAllByRole("button", { name: "Fix KU-101" }))[0]!);
    const price = await screen.findByLabelText("Price (₹)");
    await user.clear(price);
    await user.type(price, "abc");
    await user.click(screen.getByRole("button", { name: "Validate" }));

    expect(
      await screen.findByText("price must be a positive number of rupees"),
    ).toBeInTheDocument();
    expect(fake.enriched).toBeNull();
  });

  it("fixes a product without a photo and re-runs its vision call", async () => {
    fake.role = "seller";
    server.use(
      http.get("*/v1/products", () =>
        HttpResponse.json(page([{ ...product, image_count: 0, status: "needs_photo" }])),
      ),
    );
    const user = userEvent.setup();
    renderAt("/products");

    await user.click((await screen.findAllByRole("button", { name: "Fix KU-101" }))[0]!);
    await user.upload(await screen.findByLabelText("Photos for this product"), [
      new File(["png"], "front.png", { type: "image/png" }),
    ]);
    await user.click(screen.getByRole("button", { name: "Validate" }));

    await waitFor(() => {
      expect(fake.enriched).toBe("1");
    });
    expect(fake.productSaved).toBe("1499");
    expect(fake.photoAdded).toBe("1");
  });

  it("shows a reviewer the rejected rows without the fix form", async () => {
    renderAt("/products");

    expect((await screen.findAllByText("Not loaded")).length).toBeGreaterThan(0);
    expect(screen.queryByRole("button", { name: /Fix row/ })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Discard all/ })).not.toBeInTheDocument();
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
    await user.click(screen.getByRole("tab", { name: /Not approved/ }));
    await user.click(screen.getByRole("button", { name: "Approve 1 selected" }));

    expect(await screen.findByText("1 approved, 1 skipped")).toBeInTheDocument();
    expect(fake.approved.has("11")).toBe(true);
  });

  it("approves every passing listing in one click", async () => {
    const user = userEvent.setup();
    renderAt("/review");

    await user.click(await screen.findByRole("button", { name: "Approve all passing (1)" }));

    expect(await screen.findByText("1 approved, 1 skipped")).toBeInTheDocument();
    expect(fake.approved.has("11")).toBe(true);
    expect(fake.approved.has("12")).toBe(false);
  });

  it("selects every passing listing shown, then clears the selection", async () => {
    const user = userEvent.setup();
    renderAt("/review");

    await user.click(
      await screen.findByRole("checkbox", { name: "Select every passing listing in the table" }),
    );

    expect(screen.getByRole("button", { name: "Approve 1 selected" })).toBeEnabled();
    expect(screen.getByText("· 1 selected")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Clear" }));
    expect(screen.getByRole("button", { name: "Approve 0 selected" })).toBeDisabled();
  });

  it("shows one row per product with its photo, cost and triage flags", async () => {
    renderAt("/review");

    const photos = await screen.findAllByRole("img", { name: "KU-101" });
    expect(photos[0]).toHaveAttribute("src", expect.stringContaining("/v1/products/1/image"));
    expect(screen.getAllByText("Low confidence 42%").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Missing attributes").length).toBeGreaterThan(0);
    expect(screen.getAllByText("AI $0.0092").length).toBeGreaterThan(0);
  });

  it("filters to products with compliance errors", async () => {
    const user = userEvent.setup();
    renderAt("/review");

    await user.click(await screen.findByRole("tab", { name: /Compliance errors/ }));

    expect(screen.getAllByText("1 rule failing").length).toBeGreaterThan(0);
    await user.click(screen.getByRole("tab", { name: /Low confidence/ }));
    expect(screen.getAllByText("Low confidence 42%").length).toBeGreaterThan(0);
  });

  it("re-runs the rules as the reviewer types", async () => {
    const user = userEvent.setup();
    renderAt("/review");
    const links = await screen.findAllByRole("button", { name: "Navy Kurta amazon_style" });
    await user.click(links[0]!);
    const title = await screen.findByLabelText("Title");

    await user.clear(title);
    await user.type(title, "Guaranteed navy kurta");

    expect(
      await screen.findByText("for your unsaved changes", {}, { timeout: 3000 }),
    ).toBeInTheDocument();
    expect(screen.getByText('Title uses "guaranteed".')).toBeInTheDocument();
    expect(fake.version).toBe(2);
  });

  it("sends a seller who opens review to products", async () => {
    fake.role = "seller";

    renderAt("/review");

    expect(await screen.findByRole("heading", { name: "Products" })).toBeInTheDocument();
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

  it("sends a seller who opens export to products", async () => {
    fake.role = "seller";

    renderAt("/export");

    expect(await screen.findByRole("heading", { name: "Products" })).toBeInTheDocument();
  });

  it("sends a new export to the seller", async () => {
    fake.approved.add("11");
    const user = userEvent.setup();
    renderAt("/export");

    await user.click(await screen.findByRole("button", { name: "Export 1 approved listings" }));
    await user.click(await screen.findByRole("button", { name: "Send export 9 to the seller" }));

    expect(await screen.findByText("Sent to seller")).toBeInTheDocument();
    expect(fake.sent).toBe(true);
    expect(
      screen.queryByRole("button", { name: "Send export 9 to the seller" }),
    ).not.toBeInTheDocument();
  });

  it("says why an export was not sent", async () => {
    fake.approved.add("11");
    server.use(http.post("*/v1/exports/9/send", () => err(404, "not_found", "No such export.")));
    const user = userEvent.setup();
    renderAt("/export");

    await user.click(await screen.findByRole("button", { name: "Export 1 approved listings" }));
    await user.click(await screen.findByRole("button", { name: "Send export 9 to the seller" }));

    expect(await screen.findByText("No such export. (request req_test)")).toBeInTheDocument();
  });

  it("shows a seller the files a reviewer sent", async () => {
    fake.role = "seller";
    fake.sent = true;

    renderAt("/received");

    expect(
      await screen.findByRole("link", { name: /Amazon-style CSV \(1 rows\)/ }),
    ).toHaveAttribute("href", expect.stringContaining("/v1/exports/9/files/amazon_style"));
    expect(screen.getByText(/by reviewer@example.com/)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Send/ })).not.toBeInTheDocument();
  });

  it("tells a seller when nothing was sent yet", async () => {
    fake.role = "seller";

    renderAt("/received");

    expect(await screen.findByText("Nothing sent yet")).toBeInTheDocument();
  });

  it("sends a reviewer who opens received files to products", async () => {
    renderAt("/received");

    expect(await screen.findByRole("heading", { name: "Products" })).toBeInTheDocument();
  });

  it("lists the channels, including one switched off", async () => {
    renderAt("/channels");

    expect(await screen.findByText(/title_max_length must be positive/)).toBeInTheDocument();
  });

  it("switches to the dark theme and remembers it", async () => {
    const user = userEvent.setup();
    renderAt("/channels");

    await user.click(await screen.findByRole("button", { name: "Use the dark theme" }));

    expect(document.documentElement).toHaveClass("dark");
    expect(localStorage.getItem("catalift.theme")).toBe("dark");
    await user.click(screen.getByRole("button", { name: "Use the light theme" }));
    expect(document.documentElement).not.toHaveClass("dark");
  });

  it("shows a reviewer only review, export, channels and products", async () => {
    renderAt("/upload");

    expect(await screen.findByRole("heading", { name: "Products" })).toBeInTheDocument();
    const nav = screen.getByRole("navigation", { name: "Catalift" });
    expect(
      within(nav)
        .getAllByRole("link")
        .map((a) => a.textContent),
    ).toEqual(["Products", "Review", "Export", "Channels"]);
  });

  it("shows a seller only upload, products, brand voice and received files", async () => {
    fake.role = "seller";

    renderAt("/products");

    const nav = await screen.findByRole("navigation", { name: "Catalift" });
    expect(
      within(nav)
        .getAllByRole("link")
        .map((a) => a.textContent),
    ).toEqual(["Upload", "Products", "Brand voice", "Received files"]);
  });

  it("lets a reviewer change a channel's rules and reports the re-check", async () => {
    const user = userEvent.setup();
    renderAt("/channels");

    await screen.findByText("Amazon-style");
    await user.click(screen.getByRole("button", { name: /Edit rules for Amazon-style/ }));
    const words = screen.getByLabelText("Banned words and phrases, one per line");
    await user.type(words, "{enter}cheapest");
    await user.click(screen.getByRole("checkbox", { name: "fit" }));
    await user.click(screen.getByRole("button", { name: "Save rules" }));

    expect(
      await screen.findByText(/2 listings checked again; 1 approvals cleared/),
    ).toBeInTheDocument();
    expect(fake.bannedSaved).toEqual(["sale", "cheapest"]);
    expect(screen.getByText("Changed by reviewer@example.com")).toBeInTheDocument();
  });

  it("explains a rule edit that lost a race", async () => {
    server.use(
      http.patch("*/v1/channels/amazon_style", () => err(409, "version_conflict", "Changed.")),
    );
    const user = userEvent.setup();
    renderAt("/channels");

    await screen.findByText("Amazon-style");
    await user.click(screen.getByRole("button", { name: /Edit rules for Amazon-style/ }));
    await user.click(screen.getByRole("button", { name: "Save rules" }));

    expect(await screen.findByText(/Someone else changed these rules first/)).toBeInTheDocument();
  });

  it("sends a seller who opens channels to products", async () => {
    fake.role = "seller";

    renderAt("/channels");

    expect(await screen.findByRole("heading", { name: "Products" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Edit rules/ })).not.toBeInTheDocument();
  });

  it("navigates between pages from the frame", async () => {
    fake.role = "seller";
    const user = userEvent.setup();
    renderAt("/channels");

    const nav = await screen.findByRole("navigation", { name: "Catalift" });
    await user.click(within(nav).getByRole("link", { name: "Upload" }));

    expect(await screen.findByRole("heading", { name: "Upload a launch" })).toBeInTheDocument();
  });
});

describe("only the newest upload", () => {
  const asked: string[] = [];
  const record =
    (path: string) =>
    ({ request }: { request: Request }) => {
      asked.push(`${path}${new URL(request.url).search}`);
    };

  it("scopes review, brand voice, export and received files to it", async () => {
    asked.length = 0;
    server.events.on("request:start", ({ request }) => {
      const u = new URL(request.url);
      if (request.method === "GET") record(u.pathname)({ request });
    });
    renderAt("/review");
    await screen.findAllByText("Not approved");
    fake.role = "seller";
    renderAt("/brands");
    await screen.findAllByRole("button", { name: "Save voice" });
    server.events.removeAllListeners();

    for (const path of ["/v1/listings", "/v1/products", "/v1/brands"]) {
      expect(asked.some((x) => x.startsWith(path))).toBe(true);
      expect(
        asked.filter((a) => a.startsWith(path)).every((a) => a.includes("filter[upload_id]=3")),
      ).toBe(true);
    }
  });

  it("exports only the newest upload", async () => {
    let body: unknown = null;
    fake.approved.add("11");
    server.use(
      http.post("*/v1/exports", async ({ request }) => {
        body = await request.json();
        fake.exported = true;
        return HttpResponse.json({ ...exportNine, sent_at: null, sent_by: null }, { status: 201 });
      }),
    );
    const user = userEvent.setup();
    renderAt("/export");

    await user.click(await screen.findByRole("button", { name: "Export 1 approved listings" }));

    await waitFor(() => {
      expect(body).toEqual({ upload_id: "3" });
    });
  });

  it("shows nothing old before the first upload", async () => {
    fake.role = "seller";
    fake.rowFixed = true;
    let asked = "";
    server.use(
      http.get("*/v1/uploads/latest", () => err(404, "not_found", "No such record.")),
      http.get("*/v1/products", ({ request }) => {
        asked = new URL(request.url).search;
        return HttpResponse.json(page([]));
      }),
    );

    renderAt("/products");

    expect(await screen.findByText("No products yet")).toBeInTheDocument();
    expect(asked).not.toContain("upload_id");
  });
});

describe("products order", () => {
  // Regression: coming back to Products from the sidebar listed every upload
  // by SKU, so the latest upload's rows sat on a later page.
  it("shows the latest upload first when opened from the sidebar", async () => {
    fake.role = "seller";
    fake.rowFixed = true;
    server.use(
      http.get("*/v1/products", () =>
        HttpResponse.json(
          page([
            { ...product, id: "1", sku: "AA-100", created_at: "2026-10-04T08:00:00Z" },
            { ...product, id: "2", sku: "ZZ-200", created_at: "2026-10-04T08:00:00Z" },
            { ...product, id: "9", sku: "HL-2101", created_at: "2026-10-07T09:00:00Z" },
          ]),
        ),
      ),
    );
    const user = userEvent.setup();
    renderAt("/upload");

    const nav = await screen.findByRole("navigation", { name: "Catalift" });
    await user.click(within(nav).getByRole("link", { name: "Products" }));

    const table = await screen.findByRole("table");
    const skus = within(table)
      .getAllByRole("row")
      .slice(1)
      .map((r) => r.querySelector("td")?.textContent)
      .filter((t) => t && /^[A-Z]{2}-\d+$/.test(t));
    expect(skus).toEqual(["HL-2101", "AA-100", "ZZ-200"]);
  });
});

describe("brand voice", () => {
  it("saves a brand's tone and words to avoid", async () => {
    fake.role = "seller";
    const user = userEvent.setup();
    renderAt("/brands");

    await user.type(await screen.findByLabelText("Tone"), "Warm and plain");
    await user.type(screen.getByLabelText("Words to avoid"), "cheap{enter} best ever ,cheap2");
    await user.click(screen.getByRole("button", { name: "Save voice" }));

    expect(
      await screen.findByText("Indigo Loom's listings were checked for 3 words to avoid."),
    ).toBeInTheDocument();
    expect(fake.brandSaved).toEqual({
      voice_note: "Warm and plain",
      words_to_avoid: ["cheap", "best ever", "cheap2"],
    });
  });

  it("clears the tone when it is emptied", async () => {
    fake.role = "seller";
    fake.brandSaved = { voice_note: "Loud", words_to_avoid: [] };
    const user = userEvent.setup();
    renderAt("/brands");

    await user.clear(await screen.findByDisplayValue("Loud"));
    await user.click(screen.getByRole("button", { name: "Save voice" }));

    expect(await screen.findByText("Indigo Loom's voice is saved.")).toBeInTheDocument();
    expect(fake.brandSaved).toEqual({ voice_note: null, words_to_avoid: [] });
  });

  it("shows why a brand's voice was not saved", async () => {
    fake.role = "seller";
    server.use(
      http.patch("*/v1/brands/1", () =>
        err(400, "invalid_request", "words_to_avoid: at most 50 words or phrases"),
      ),
    );
    const user = userEvent.setup();
    renderAt("/brands");

    await user.click(await screen.findByRole("button", { name: "Save voice" }));

    expect(
      await screen.findByText("words_to_avoid: at most 50 words or phrases (request req_test)"),
    ).toBeInTheDocument();
  });

  it("sends a reviewer who opens brand voice to products", async () => {
    renderAt("/brands");

    expect(await screen.findByRole("heading", { name: "Products" })).toBeInTheDocument();
  });
});
