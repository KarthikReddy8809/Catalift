import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { useEffect, useState } from "react";

import { budgetQueryOptions } from "@/features/shell/api";
import { errorBody } from "@/lib/api";

import {
  brandsWithoutVoiceQueryOptions,
  catalogueKeys,
  productsQueryOptions,
  resumeRun,
  runQueryOptions,
  startRun,
  type Product,
  type Run,
} from "../api";
import { ProductsView, type ProductRow, type ProductsViewProps } from "../components/ProductsView";
import { readLaunch, rememberLaunch } from "../launch";

function attributeLine(p: Product): string | undefined {
  const a = p.attributes;
  const parts = [a.colour, a.pattern, a.sleeve, a.neckline, a.fit].filter(
    (v): v is string => v !== null && v !== "unknown",
  );
  return parts.length > 0 ? parts.join(", ") : undefined;
}

function toRow(p: Product): ProductRow {
  return {
    sku: p.sku,
    brand: p.brand.name,
    category: p.category,
    priceMinor: p.price_minor,
    imageCount: p.image_count,
    detection: p.attributes.detection_status,
    detectionError: p.attributes.detection_error ?? undefined,
    attributes: attributeLine(p),
    aiCostMicroUsd: p.ai_cost_micro_usd,
  };
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
export function ProductsPage({ upload }: { upload?: string | undefined }) {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [launch, setLaunch] = useState(() => {
    const saved = readLaunch();
    return upload && upload !== saved.uploadId ? { uploadId: upload } : saved;
  });
  const run = useQuery({ ...runQueryOptions(launch.runId ?? ""), enabled: !!launch.runId });
  // Products and their cost follow the run while it writes.
  const running =
    !!run.data && (run.data.detection.pending ?? 0) + (run.data.listings.queued ?? 0) > 0;
  const products = useQuery({
    ...productsQueryOptions(launch.uploadId),
    refetchInterval: running ? 3_000 : false,
  });
  // Each new run count may mean new attributes or cost, so the products follow it.
  const runUpdatedAt = run.dataUpdatedAt;
  useEffect(() => {
    if (runUpdatedAt > 0) {
      void queryClient.invalidateQueries({ queryKey: catalogueKeys.products(launch.uploadId) });
    }
  }, [runUpdatedAt, queryClient, launch.uploadId]);
  const brands = useQuery(brandsWithoutVoiceQueryOptions());
  const budget = useQuery(budgetQueryOptions());

  const onRun = (r: Run) => {
    const next = { ...launch, runId: r.id };
    setLaunch(next);
    rememberLaunch(next);
    queryClient.setQueryData(catalogueKeys.run(r.id), r);
    void queryClient.invalidateQueries({ queryKey: ["budget"] });
  };
  const start = useMutation({
    mutationFn: (neutral: boolean) => startRun(neutral, launch.uploadId),
    onSuccess: onRun,
  });
  const resume = useMutation({ mutationFn: () => resumeRun(launch.runId ?? ""), onSuccess: onRun });

  const failure = products.error ?? start.error ?? resume.error;
  const props: ProductsViewProps = {
    status: products.isPending ? "loading" : products.isError ? "error" : "ready",
    products: (products.data ?? []).map(toRow),
    brandsWithoutVoice: brands.data ?? [],
    budgetBlocked: !!budget.data?.blocked_at,
    uploadName: launch.fileName ?? (launch.uploadId ? `Upload ${launch.uploadId}` : "All uploads"),
    onGenerate: (neutral) => {
      start.mutate(neutral);
    },
    onResume: () => {
      resume.mutate();
    },
    onUploadLaunch: () => {
      void navigate({ to: "/upload" });
    },
  };
  if (run.data) props.run = toCounts(run.data);
  const body = errorBody(failure);
  if (body?.requestId) props.requestId = body.requestId;
  return (
    <>
      <ProductsView {...props} />
      {start.isError || resume.isError ? (
        <p role="alert" className="mt-4 text-sm text-destructive">
          {body?.message ?? "The run could not start. Try again."}
        </p>
      ) : null}
    </>
  );
}
