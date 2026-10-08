import { useQuery } from "@tanstack/react-query";

import { useCurrentUpload } from "@/features/catalogue/current";
import { channelsQueryOptions } from "@/features/channels/api";
import { errorBody } from "@/lib/api";

import { exportsQueryOptions } from "../api";
import { ReceivedView } from "../components/ReceivedView";
import { toListItem } from "../items";

/** ReceivedPage (sellers): the exports reviewers sent, ready to download (ADR-0012). */
export function ReceivedPage() {
  // Only files sent for the newest upload.
  const current = useCurrentUpload();
  const exports = useQuery({ ...exportsQueryOptions(current.id), enabled: current.ready });
  const channels = useQuery(channelsQueryOptions(current.id));
  const nameOf = (id: string) => (channels.data ?? []).find((c) => c.id === id)?.name ?? id;
  return (
    <ReceivedView
      status={exports.isPending ? "loading" : exports.isError ? "error" : "ready"}
      exports={(exports.data ?? []).map((e) => toListItem(e, nameOf))}
      requestId={errorBody(exports.error)?.requestId}
    />
  );
}
