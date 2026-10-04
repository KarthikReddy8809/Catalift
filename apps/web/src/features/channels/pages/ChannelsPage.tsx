import { useQuery } from "@tanstack/react-query";

import { errorBody } from "@/lib/api";

import { channelsQueryOptions } from "../api";
import { ChannelsView, type ChannelsViewProps } from "../components/ChannelsView";

const when = new Intl.DateTimeFormat("en-IN", {
  dateStyle: "medium",
  timeStyle: "short",
  timeZone: "Asia/Kolkata",
});

/** ChannelsPage: each channel's rules, and why a channel is off (US-00-005, D8). */
export function ChannelsPage() {
  const channels = useQuery(channelsQueryOptions());
  const props: ChannelsViewProps = {
    status: channels.isPending ? "loading" : channels.isError ? "error" : "ready",
    channels: (channels.data ?? []).map((c) => ({
      id: c.id,
      name: c.name,
      enabled: c.enabled,
      loadError: c.load_error,
      titleMaxLength: c.title_max_length,
      requiredAttributes: c.required_attributes,
      bannedWordCount: c.banned_words.length,
      lastRecheck: c.last_recheck
        ? {
            listingsRechecked: c.last_recheck.listings_rechecked,
            approvalsCleared: c.last_recheck.approvals_cleared,
            when: `${when.format(new Date(c.last_recheck.created_at))} IST`,
          }
        : null,
    })),
  };
  const body = errorBody(channels.error);
  if (body?.requestId) props.requestId = body.requestId;
  return <ChannelsView {...props} />;
}
