import { useMutation, useQuery, useQueryClient, useSuspenseQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { useEffect, useState } from "react";

import { sessionQueryOptions } from "@/features/auth/api";
import { budgetQueryOptions } from "@/features/shell/api";
import { ApiError, errorBody } from "@/lib/api";

import {
  brandsWithoutVoiceQueryOptions,
  catalogueKeys,
  productsQueryOptions,
  discardRow,
  discardRows,
  enrichProduct,
  fixRow,
  resumeRun,
  updateProduct,
  rowErrorsQueryOptions,
  uploadProductPhotos,
  runQueryOptions,
  startRun,
  type Product,
  type Run,
} from "../api";
import {
  ProductFixDrawer,
  type FixOutcome,
  type FixTarget,
  type FixValues,
} from "../components/ProductFixDrawer";
import {
  ProductsView,
  type ProductAttribute,
  type ProductRow,
  type ProductsViewProps,
  type RejectedRow,
} from "../components/ProductsView";
import { useCurrentUpload } from "../current";
import { readLaunch, rememberLaunch } from "../launch";
import { newestFirst } from "../order";

/**
 * failureText says which call failed and how, so the seller (and whoever
 * runs the server) can act: a 404 or 405 from a route this page relies on
 * almost always means the API is older than the page and needs a restart.
 */
function failureText(err: unknown): string {
  const body = errorBody(err);
  if (err instanceof ApiError && (err.status === 404 || err.status === 405) && !body?.code) {
    return `${err.message}. The API does not have this action yet: restart it (make dev) so it runs the latest code.`;
  }
  if (body?.message)
    return body.requestId ? `${body.message} (request ${body.requestId})` : body.message;
  if (err instanceof ApiError) return `${err.message}.`;
  return err instanceof Error ? err.message : "Something failed. Try again.";
}

/** What the fix drawer has open. */
type Fixing = { kind: "row"; row: RejectedRow } | { kind: "product"; product: ProductRow };

const NO_PHOTO = "No photo yet, so the AI cannot read this product.";

const PROBLEM: Record<string, string> = {
  needs_photo: NO_PHOTO,
  failed: "The AI could not finish this product.",
  budget_exhausted: "The AI budget ran out before this product was done.",
};

function drawerTarget(f: Fixing): FixTarget {
  if (f.kind === "row") {
    const r = f.row;
    return {
      key: `row:${r.uploadId}:${String(r.rowNumber)}`,
      title: `Row ${String(r.rowNumber)} of ${r.fileName}`,
      problem: r.reason,
      values: { sku: r.sku, category: r.category, brand: r.brand, price: r.price },
      photoCount: 0,
    };
  }
  const p = f.product;
  return {
    key: `product:${p.id ?? p.sku}`,
    title: p.sku,
    problem:
      p.detectionError ??
      PROBLEM[p.status ?? ""] ??
      (p.imageCount === 0 ? NO_PHOTO : "Check the details and run the AI again."),
    values: {
      sku: p.sku,
      category: p.category,
      brand: p.brand,
      price: String(p.priceMinor / 100),
    },
    photoCount: p.imageCount,
  };
}

const ATTRIBUTE_NAMES = [
  ["colour", "Colour"],
  ["pattern", "Pattern"],
  ["sleeve", "Sleeve"],
  ["neckline", "Neckline"],
  ["fit", "Fit"],
] as const;

/** attributeList keeps what the AI read; "unknown" and empty fields are left out. */
function attributeList(p: Product): ProductAttribute[] {
  return ATTRIBUTE_NAMES.flatMap(([field, name]) => {
    const value = p.attributes[field];
    return value && value !== "unknown" ? [{ name, value }] : [];
  });
}

function toRow(p: Product): ProductRow {
  return {
    id: p.id,
    uploadId: p.upload_id,
    sku: p.sku,
    brand: p.brand.name,
    category: p.category,
    priceMinor: p.price_minor,
    imageCount: p.image_count,
    detection: p.attributes.detection_status,
    status: p.status,
    detectionError: p.attributes.detection_error ?? undefined,
    attributes: attributeList(p),
    aiCostMicroUsd: p.ai_cost_micro_usd,
  };
}

/** loadProblem says why the list did not load, in words the seller can act on. */
function loadProblem(err: unknown): string {
  if (err instanceof ApiError && err.code === "invalid_response") {
    return "The server sent products in an older shape. Restart the API so it runs the latest code.";
  }
  if (err instanceof ApiError && err.code === "network") {
    return "The API did not answer. Check that the server is running.";
  }
  return errorBody(err)?.message ?? (err instanceof Error ? err.message : "Unknown error.");
}

function toCounts(r: Run): NonNullable<ProductsViewProps["run"]> {
  return {
    detection: {
      pending: r.detection.pending ?? 0,
      done: r.detection.done ?? 0,
      failed: r.detection.failed,
      stopped_budget: r.detection.stopped_budget,
    },
    listings: {
      queued: r.listings.queued ?? 0,
      generated: r.listings.generated ?? 0,
      failed: r.listings.failed,
      stopped_budget: r.listings.stopped_budget,
    },
  };
}

/** ProductsPage: the upload's products, their AI cost, and the generation run. */
export function ProductsPage({ upload: named }: { upload?: string | undefined }) {
  // Only the newest upload's records show, unless the address names one;
  // older uploads stay in the database.
  const current = useCurrentUpload();
  const upload = named ?? current.id;
  const scopeKnown = !!named || current.ready;
  const { data: session } = useSuspenseQuery(sessionQueryOptions());
  const seller = session?.user.role === "seller";
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  // The saved launch only brings back the run's progress; the list follows
  // the upload in scope.
  const [launch, setLaunch] = useState(readLaunch);
  const run = useQuery({ ...runQueryOptions(launch.runId ?? ""), enabled: !!launch.runId });
  // Products and their cost follow the run while it writes.
  const running =
    !!run.data && (run.data.detection.pending ?? 0) + (run.data.listings.queued ?? 0) > 0;
  const products = useQuery({
    ...productsQueryOptions(upload),
    enabled: scopeKnown,
    refetchInterval: running ? 3_000 : false,
  });
  // Each new run count may mean new attributes or cost, so the products follow it.
  const runUpdatedAt = run.dataUpdatedAt;
  useEffect(() => {
    if (runUpdatedAt > 0) {
      void queryClient.invalidateQueries({ queryKey: catalogueKeys.products(upload) });
    }
  }, [runUpdatedAt, queryClient, upload]);
  const brands = useQuery({ ...brandsWithoutVoiceQueryOptions(upload), enabled: scopeKnown });
  const rowErrors = useQuery({ ...rowErrorsQueryOptions(upload), enabled: scopeKnown });
  const [fixTarget, setFixTarget] = useState<Fixing | undefined>();
  const [fixOutcome, setFixOutcome] = useState<FixOutcome | undefined>();
  const openFix = (f: Fixing) => {
    setFixOutcome(undefined);
    setFixTarget(f);
  };
  // Validate: check and save the details, upload the photos, then run the one
  // vision call for this product. Each step stops at the first problem and
  // says what it was, inside the drawer.
  const validate = useMutation({
    mutationFn: async (v: {
      target: Fixing;
      values: FixValues;
      photos: File[];
    }): Promise<FixOutcome> => {
      let productId = v.target.kind === "product" ? v.target.product.id : undefined;
      if (v.target.kind === "row") {
        const res = await fixRow(v.target.row.uploadId, v.target.row.rowNumber, v.values);
        if (res.status === "rejected" || !res.product_id) {
          return { tone: "error", message: res.reason ?? "The row was not loaded." };
        }
        productId = res.product_id;
      } else if (productId) {
        const res = await updateProduct(productId, v.values);
        if (res.status === "rejected") {
          return { tone: "error", message: res.reason ?? "The details were not saved." };
        }
      }
      if (!productId) return { tone: "error", message: "The product could not be found." };
      let photoCount = v.target.kind === "product" ? v.target.product.imageCount : 0;
      if (v.photos.length > 0) {
        const res = await uploadProductPhotos(productId, v.photos);
        photoCount += res.attached.length;
        if (res.rejected_files.length > 0) {
          const refused = res.rejected_files.map((f) => `${f.file_name} (${f.reason})`).join(", ");
          if (res.attached.length === 0)
            return { tone: "error", message: `Details saved; photos not added: ${refused}.` };
        }
      }
      if (photoCount === 0) {
        return {
          tone: "error",
          message: "Details saved. Add a photo so the AI can read the garment.",
        };
      }
      const run = await enrichProduct(productId);
      onRun(run);
      return {
        tone: "success",
        message: "Saved. The AI is reading this product again; its status updates below.",
      };
    },
    onSuccess: (outcome) => {
      setFixOutcome(outcome);
      if (outcome.tone === "success") setFixTarget(undefined);
    },
    onError: (err) => {
      setFixOutcome({ tone: "error", message: failureText(err) });
    },
    onSettled: () => void queryClient.invalidateQueries({ queryKey: catalogueKeys.all }),
  });
  const discardOne = useMutation({
    mutationFn: (row: RejectedRow) => discardRow(row.uploadId, row.rowNumber),
    onSuccess: () => {
      setFixTarget(undefined);
    },
    onError: (err) => {
      setFixOutcome({ tone: "error", message: failureText(err) });
    },
    onSettled: () => void queryClient.invalidateQueries({ queryKey: catalogueKeys.all }),
  });
  const discardAll = useMutation({
    mutationFn: () => discardRows(upload),
    onSettled: () => void queryClient.invalidateQueries({ queryKey: catalogueKeys.all }),
  });
  const budget = useQuery(budgetQueryOptions());

  const onRun = (r: Run) => {
    const next = { ...launch, runId: r.id };
    setLaunch(next);
    rememberLaunch(next);
    queryClient.setQueryData(catalogueKeys.run(r.id), r);
    void queryClient.invalidateQueries({ queryKey: ["budget"] });
  };
  const start = useMutation({
    mutationFn: (neutral: boolean) => startRun(neutral, upload),
    onSuccess: onRun,
  });
  const resume = useMutation({ mutationFn: () => resumeRun(launch.runId ?? ""), onSuccess: onRun });

  const rejectedRows: RejectedRow[] = (rowErrors.data?.data ?? []).map((e) => ({
    uploadId: e.upload_id,
    fileName: e.file_name,
    rowNumber: e.row_number,
    sku: e.sku ?? "",
    category: e.category ?? "",
    brand: e.brand ?? "",
    price: e.price ?? "",
    reason: e.reason,
  }));
  const failure = products.error ?? start.error ?? resume.error;
  const props: ProductsViewProps = {
    status: products.isPending ? "loading" : products.isError ? "error" : "ready",
    products: [...(products.data ?? [])].sort(newestFirst).map(toRow),
    brandsWithoutVoice: brands.data ?? [],
    budgetBlocked: !!budget.data?.blocked_at,
    uploadName: upload
      ? upload === current.id && current.fileName
        ? current.fileName
        : upload === launch.uploadId && launch.fileName
          ? launch.fileName
          : `Upload ${upload}`
      : "All products",
    onGenerate: (neutral) => {
      start.mutate(neutral);
    },
    ...(seller
      ? {
          onOpenFix: (row: ProductRow) => {
            openFix({ kind: "product", product: row });
          },
        }
      : {}),
    rejected: rejectedRows,
    discarding: discardAll.isPending,
    ...(seller
      ? {
          onOpenRejected: (row: RejectedRow) => {
            openFix({ kind: "row", row });
          },
          onDiscardAll: () => {
            discardAll.mutate();
          },
        }
      : {}),
    onResume: () => {
      resume.mutate();
    },
    // Only sellers upload; a reviewer works on what is already there.
    ...(seller
      ? {
          onUploadLaunch: () => {
            void navigate({ to: "/upload" });
          },
        }
      : {}),
  };
  if (run.data) props.run = toCounts(run.data);
  const body = errorBody(failure);
  if (body?.requestId) props.requestId = body.requestId;
  if (products.error) props.errorMessage = loadProblem(products.error);
  return (
    <>
      <ProductsView {...props} />
      <ProductFixDrawer
        target={fixTarget ? drawerTarget(fixTarget) : undefined}
        categories={rowErrors.data?.categories ?? []}
        busy={validate.isPending}
        outcome={fixOutcome}
        onValidate={(values, files) => {
          if (fixTarget) validate.mutate({ target: fixTarget, values, photos: files });
        }}
        onClose={() => {
          setFixTarget(undefined);
        }}
        {...(fixTarget?.kind === "row"
          ? {
              onDiscard: () => {
                discardOne.mutate(fixTarget.row);
              },
            }
          : {})}
      />
      {start.isError || resume.isError ? (
        <p role="alert" className="mt-4 text-sm text-destructive">
          {body?.message ?? "The run could not start. Try again."}
        </p>
      ) : null}
    </>
  );
}
