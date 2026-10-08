import { useMutation, useQuery, useQueryClient, useSuspenseQuery } from "@tanstack/react-query";

import { sessionQueryOptions } from "@/features/auth/api";
import { useCurrentUpload } from "@/features/catalogue/current";
import { formatIst } from "@/features/shell/format";
import { ApiError, errorBody } from "@/lib/api";

import { channelsQueryOptions, saveRules } from "../api";
import { ChannelsView, type ChannelsViewProps } from "../components/ChannelsView";

const ist = formatIst;

/** ChannelsPage: each channel's rules, why a channel is off (D8), and the reviewer's editor. */
export function ChannelsPage() {
  const { data: session } = useSuspenseQuery(sessionQueryOptions());
  const queryClient = useQueryClient();
  // Listing counts are for the newest upload, like every other screen.
  const current = useCurrentUpload();
  const channels = useQuery({ ...channelsQueryOptions(current.id), enabled: current.ready });
  const save = useMutation({
    mutationFn: (v: Parameters<typeof saveRules>) => saveRules(...v),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["channels"] });
      // A rule change can clear approvals and change compliance badges.
      void queryClient.invalidateQueries({ queryKey: ["listings"] });
      void queryClient.invalidateQueries({ queryKey: ["catalogue"] });
    },
    onError: (err) => {
      // Someone else saved first: show their rules so the reviewer edits those.
      if (err instanceof ApiError && err.status === 409) {
        void queryClient.invalidateQueries({ queryKey: ["channels"] });
      }
    },
  });

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
      bannedWords: c.banned_words,
      configHash: c.config_hash,
      lastEdit: c.last_edit ? { by: c.last_edit.by, when: ist(c.last_edit.at) } : null,
      lastRecheck: c.last_recheck
        ? {
            listingsRechecked: c.last_recheck.listings_rechecked,
            approvalsCleared: c.last_recheck.approvals_cleared,
            when: ist(c.last_recheck.created_at),
          }
        : null,
    })),
    saving: save.isPending,
  };
  if (session?.user.role === "reviewer") {
    props.onSave = (row, edit) => {
      save.mutate([
        row.id,
        row.configHash ?? "",
        {
          title_max_length: edit.titleMaxLength,
          required_attributes: edit.requiredAttributes,
          banned_words: edit.bannedWords,
        },
      ]);
    };
  }
  if (save.data) {
    props.saved = {
      channel: save.data.channel,
      rechecked: save.data.listings_rechecked,
      cleared: save.data.approvals_cleared,
    };
  }
  if (save.error) {
    props.saveError =
      save.error instanceof ApiError && save.error.status === 409
        ? "Someone else changed these rules first. The page now shows theirs; make your change again."
        : (errorBody(save.error)?.message ?? "Try again.");
  }
  const body = errorBody(channels.error);
  if (body?.requestId) props.requestId = body.requestId;
  return <ChannelsView {...props} />;
}
