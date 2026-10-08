import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { catalogueKeys } from "@/features/catalogue/api";
import { useCurrentUpload } from "@/features/catalogue/current";
import { errorBody } from "@/lib/api";

import { brandKeys, brandsQueryOptions, saveBrandVoice } from "../api";
import { BrandVoiceView, type BrandOutcome } from "../components/BrandVoiceView";

/** BrandVoicePage (sellers): each brand's tone and words to avoid (ADR-0012). */
export function BrandVoicePage() {
  const queryClient = useQueryClient();
  // Only the brands the newest upload names.
  const current = useCurrentUpload();
  const brands = useQuery({ ...brandsQueryOptions(current.id), enabled: current.ready });
  const [outcome, setOutcome] = useState<BrandOutcome | undefined>();
  const save = useMutation({
    mutationFn: (v: { id: string; tone: string; wordsToAvoid: string[] }) =>
      saveBrandVoice(v.id, { tone: v.tone, wordsToAvoid: v.wordsToAvoid }),
    onSuccess: (b) => {
      setOutcome({
        brandId: b.id,
        tone: "success",
        message:
          b.words_to_avoid.length > 0
            ? `${b.name}'s listings were checked for ${String(b.words_to_avoid.length)} word${b.words_to_avoid.length === 1 ? "" : "s"} to avoid.`
            : `${b.name}'s voice is saved.`,
      });
    },
    onError: (err, v) => {
      const body = errorBody(err);
      setOutcome({
        brandId: v.id,
        tone: "error",
        message: body?.message
          ? `${body.message}${body.requestId ? ` (request ${body.requestId})` : ""}`
          : "Try again.",
      });
    },
    onSettled: () => {
      void queryClient.invalidateQueries({ queryKey: brandKeys.all });
      // The products page asks about brands without a tone.
      void queryClient.invalidateQueries({ queryKey: catalogueKeys.all });
    },
  });
  return (
    <BrandVoiceView
      status={brands.isPending ? "loading" : brands.isError ? "error" : "ready"}
      brands={(brands.data ?? []).map((b) => ({
        id: b.id,
        name: b.name,
        tone: b.voice_note ?? "",
        wordsToAvoid: b.words_to_avoid,
        version: b.updated_at,
      }))}
      savingId={save.isPending ? save.variables.id : undefined}
      outcome={outcome}
      requestId={errorBody(brands.error)?.requestId}
      onSave={(id, tone, wordsToAvoid) => {
        setOutcome(undefined);
        save.mutate({ id, tone, wordsToAvoid });
      }}
    />
  );
}
