import { useMutation, useQuery, useQueryClient, useSuspenseQuery } from "@tanstack/react-query";

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { sessionQueryOptions } from "@/features/auth/api";
import { channelsQueryOptions } from "@/features/channels/api";
import { ApiError, errorBody } from "@/lib/api";

import { createExport, downloadHref } from "../api";
import { ExportView, type ExportViewProps } from "../components/ExportView";

/** ExportPage: readiness per channel, then one CSV per channel (US-00-010). */
export function ExportPage() {
  const queryClient = useQueryClient();
  const { data: session } = useSuspenseQuery(sessionQueryOptions());
  const channels = useQuery(channelsQueryOptions());
  const run = useMutation({
    mutationFn: createExport,
    onSettled: () => void queryClient.invalidateQueries({ queryKey: ["channels"] }),
  });

  if (session?.user.role !== "reviewer") {
    return (
      <div className="space-y-4">
        <h1 className="text-2xl font-semibold tracking-tight">Export</h1>
        <Alert role="alert">
          <AlertTitle>Export is for reviewers</AlertTitle>
          <AlertDescription>
            Listings leave Catalift only after a reviewer approves them. Ask a reviewer to export,
            or carry on in Products.
          </AlertDescription>
        </Alert>
      </div>
    );
  }

  const list = (channels.data ?? []).filter((c) => c.enabled);
  const nameOf = (id: string) => list.find((c) => c.id === id)?.name ?? id;
  const err = run.error ?? channels.error;
  let status: ExportViewProps["status"] = "ready";
  if (run.isPending) status = "exporting";
  else if (run.data) status = "done";
  else if (err instanceof ApiError && err.status === 422) status = "nothing-approved";
  else if (err) status = "error";

  const props: ExportViewProps = {
    status,
    channels: list.map((c) => ({
      channel: c.id,
      name: c.name,
      approved: c.listings_approved,
      total: c.listings_total,
    })),
    onExport: () => {
      run.mutate();
    },
  };
  if (run.data) {
    props.files = run.data.files.map((f) => ({
      channel: f.channel,
      name: nameOf(f.channel),
      rowCount: f.row_count,
      downloadUrl: downloadHref(f.download_url),
    }));
    props.skipped = run.data.skipped_channels.map((s) => ({
      name: nameOf(s.channel),
      reason: s.reason,
    }));
  }
  const body = errorBody(err);
  if (body?.requestId) props.requestId = body.requestId;
  return <ExportView {...props} />;
}
