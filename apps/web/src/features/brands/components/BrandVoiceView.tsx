import { useState } from "react";

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import { Textarea } from "@/components/ui/textarea";

import { parseWords } from "../words";

export interface BrandRow {
  id: string;
  name: string;
  tone: string;
  wordsToAvoid: string[];
  /** Changes on every save, so the form starts again from what was stored. */
  version: string;
}

/** The result of the last save of one brand. */
export interface BrandOutcome {
  brandId: string;
  tone: "error" | "success";
  message: string;
}

export interface BrandVoiceViewProps {
  status: "loading" | "ready" | "error";
  brands?: BrandRow[];
  savingId?: string | undefined;
  outcome?: BrandOutcome | undefined;
  requestId?: string | undefined;
  /** Saves one brand; absent in the design gallery. */
  onSave?: ((id: string, tone: string, wordsToAvoid: string[]) => void) | undefined;
}

function BrandForm({
  brand,
  saving,
  outcome,
  onSave,
}: {
  brand: BrandRow;
  saving: boolean;
  outcome?: BrandOutcome | undefined;
  onSave?: BrandVoiceViewProps["onSave"];
}) {
  const [tone, setTone] = useState(brand.tone);
  const [words, setWords] = useState(brand.wordsToAvoid.join("\n"));
  const toneId = `tone-${brand.id}`;
  const wordsId = `avoid-${brand.id}`;
  return (
    <form
      className="space-y-4 rounded-xl border bg-card p-5 shadow-sm"
      aria-label={`Voice of ${brand.name}`}
      onSubmit={(e) => {
        e.preventDefault();
        onSave?.(brand.id, tone, parseWords(words));
      }}
    >
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h2 className="font-semibold">{brand.name}</h2>
        {brand.tone ? null : <Badge variant="outline">No tone yet</Badge>}
      </div>
      {outcome ? (
        <Alert
          variant={outcome.tone === "error" ? "destructive" : "default"}
          role={outcome.tone === "error" ? "alert" : "status"}
        >
          <AlertTitle>{outcome.tone === "error" ? "Not saved" : "Saved"}</AlertTitle>
          <AlertDescription>{outcome.message}</AlertDescription>
        </Alert>
      ) : null}
      <div className="space-y-2">
        <Label htmlFor={toneId}>Tone</Label>
        <Textarea
          id={toneId}
          value={tone}
          maxLength={2000}
          rows={3}
          placeholder="For example: warm and plain-spoken; mention the fabric first."
          onChange={(e) => {
            setTone(e.target.value);
          }}
        />
        <p className="text-xs text-muted-foreground">
          How listings for this brand should sound. Without a tone, a run asks you to confirm a
          neutral voice.
        </p>
      </div>
      <div className="space-y-2">
        <Label htmlFor={wordsId}>Words to avoid</Label>
        <Textarea
          id={wordsId}
          value={words}
          rows={4}
          placeholder={"cheap\nbest ever"}
          onChange={(e) => {
            setWords(e.target.value);
          }}
        />
        <p className="text-xs text-muted-foreground">
          One word or phrase per line. The AI is told to avoid them, and the rules check flags any
          listing that uses one; saving re-checks this brand&rsquo;s listings.
        </p>
      </div>
      <Button type="submit" className="h-11" disabled={saving}>
        {saving ? "Saving…" : "Save voice"}
      </Button>
    </form>
  );
}

/**
 * BrandVoiceView (sellers): GET /v1/brands and PATCH /v1/brands/{id}. Each
 * brand's tone and words to avoid shape its listings (brand voice settings,
 * ADR-0012).
 */
export function BrandVoiceView({
  status,
  brands = [],
  savingId,
  outcome,
  requestId,
  onSave,
}: BrandVoiceViewProps) {
  return (
    <div className="space-y-6">
      <header className="space-y-1">
        <h1 className="text-2xl font-semibold tracking-tight">Brand voice</h1>
        <p className="max-w-prose text-muted-foreground">
          Set how each brand sounds and the words it never uses. Brands appear here once a CSV names
          them.
        </p>
      </header>
      {status === "loading" ? (
        <div className="grid gap-4 lg:grid-cols-2" aria-busy="true" aria-label="Loading brands">
          <Skeleton className="h-80 w-full" />
          <Skeleton className="h-80 w-full" />
        </div>
      ) : status === "error" ? (
        <Alert variant="destructive" role="alert">
          <AlertTitle>Brands could not be loaded</AlertTitle>
          <AlertDescription>
            Reload the page. If it keeps failing, quote {requestId ?? "the time it happened"} to the
            Catalift team.
          </AlertDescription>
        </Alert>
      ) : brands.length === 0 ? (
        <div className="space-y-2 rounded-xl border bg-card p-8 shadow-sm">
          <h2 className="font-medium">No brands yet</h2>
          <p className="max-w-prose text-muted-foreground">
            Upload a product CSV; every brand it names appears here for you to set its voice.
          </p>
        </div>
      ) : (
        <div className="grid gap-4 lg:grid-cols-2">
          {brands.map((b) => (
            <BrandForm
              key={`${b.id}:${b.version}`}
              brand={b}
              saving={savingId === b.id}
              outcome={outcome?.brandId === b.id ? outcome : undefined}
              onSave={onSave}
            />
          ))}
        </div>
      )}
    </div>
  );
}
