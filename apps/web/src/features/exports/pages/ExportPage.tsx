import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { useCurrentUpload } from "@/features/catalogue/current";
import { channelsQueryOptions } from "@/features/channels/api";
import { ApiError, errorBody } from "@/lib/api";

import { createExport, downloadHref, exportKeys, exportsQueryOptions, sendExport } from "../api";
import { ExportView, type ExportViewProps } from "../components/ExportView";
import { toListItem } from "../items";

/**
 * ExportPage (reviewers): readiness per channel, one CSV per channel
 * (US-00-010), then Send to seller for each export (ADR-0012).
 */
export function ExportPage() {
  const queryClient = useQueryClient();
  // Readiness, the export and the send list cover the newest upload only.
  const current = useCurrentUpload();
  const channels = useQuery({ ...channelsQueryOptions(current.id), enabled: current.ready });
  const history = useQuery({ ...exportsQueryOptions(current.id), enabled: current.ready });
  const run = useMutation({
    mutationFn: () => createExport(current.id),
    onSettled: () => {
      void queryClient.invalidateQueries({ queryKey: ["channels"] });
      void queryClient.invalidateQueries({ queryKey: exportKeys.all });
    },
  });
  const send = useMutation({
    mutationFn: sendExport,
    onSettled: () => void queryClient.invalidateQueries({ queryKey: exportKeys.all }),
  });

  const list = (channels.data ?? []).filter((c) => c.enabled);
  const nameOf = (id: string) => (channels.data ?? []).find((c) => c.id === id)?.name ?? id;
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
    history: (history.data ?? []).map((e) => toListItem(e, nameOf)),
    onSend: (id) => {
      send.mutate(id);
    },
    sendingId: send.isPending ? send.variables : undefined,
  };
  if (send.error) {
    const body = errorBody(send.error);
    props.sendError = body?.message
      ? `${body.message}${body.requestId ? ` (request ${body.requestId})` : ""}`
      : "Try again.";
  }
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
