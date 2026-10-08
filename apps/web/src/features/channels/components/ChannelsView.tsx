import { Pencil } from "lucide-react";
import { useState } from "react";

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Textarea } from "@/components/ui/textarea";

export interface ChannelRow {
  id: string;
  name: string;
  enabled: boolean;
  loadError: string | null;
  titleMaxLength: number | null;
  requiredAttributes: string[];
  bannedWordCount: number;
  /** The words themselves, for the editor. */
  bannedWords?: string[] | undefined;
  /** Identifies the rules in force; sent back with an edit. */
  configHash?: string | null | undefined;
  /** The last change made in the app: who, and when (already formatted). */
  lastEdit?: { by: string; when: string } | null | undefined;
  lastRecheck: { listingsRechecked: number; approvalsCleared: number; when: string } | null;
}

/** A reviewer's edit to one channel's rules. */
export interface RulesEdit {
  titleMaxLength: number;
  requiredAttributes: string[];
  bannedWords: string[];
}

export interface ChannelsViewProps {
  status: "loading" | "ready" | "error";
  channels?: ChannelRow[];
  requestId?: string;
  /** Reviewers may change the rules; sellers see them read-only. */
  onSave?: ((row: ChannelRow, edit: RulesEdit) => void) | undefined;
  saving?: boolean | undefined;
  /** What the last save did, or why it failed. */
  saved?: { channel: string; rechecked: number; cleared: number } | undefined;
  saveError?: string | undefined;
}

const ATTRIBUTES = ["colour", "pattern", "sleeve", "neckline", "fit"];

function RulesEditor({
  row,
  saving,
  onSave,
  onCancel,
}: {
  row: ChannelRow;
  saving: boolean;
  onSave: (edit: RulesEdit) => void;
  onCancel: () => void;
}) {
  const [limit, setLimit] = useState(String(row.titleMaxLength ?? 100));
  const [attrs, setAttrs] = useState<string[]>(row.requiredAttributes);
  const [words, setWords] = useState((row.bannedWords ?? []).join("\n"));
  const n = Number(limit);
  const limitOk = Number.isInteger(n) && n >= 10 && n <= 500;
  return (
    <form
      aria-labelledby="rules-editor"
      className="space-y-4 rounded-xl border bg-card p-5 shadow-sm"
      onSubmit={(e) => {
        e.preventDefault();
        if (!limitOk) return;
        onSave({
          titleMaxLength: n,
          requiredAttributes: attrs,
          bannedWords: words
            .split("\n")
            .map((w) => w.trim())
            .filter(Boolean),
        });
      }}
    >
      <h2 id="rules-editor" className="font-medium">
        Edit {row.name} rules
      </h2>
      <div className="space-y-2">
        <Label htmlFor="title-limit">Longest title, in characters (10 to 500)</Label>
        <Input
          id="title-limit"
          type="number"
          min={10}
          max={500}
          value={limit}
          aria-invalid={!limitOk}
          onChange={(e) => {
            setLimit(e.target.value);
          }}
          className="h-11 w-32"
        />
      </div>
      <fieldset className="space-y-2">
        <legend className="text-sm font-medium">Required attributes</legend>
        <div className="flex flex-wrap gap-4">
          {ATTRIBUTES.map((a) => (
            <label key={a} className="flex items-center gap-2 text-sm">
              <Checkbox
                checked={attrs.includes(a)}
                onCheckedChange={(v) => {
                  setAttrs((cur) => (v === true ? [...cur, a] : cur.filter((x) => x !== a)));
                }}
              />
              {a}
            </label>
          ))}
        </div>
      </fieldset>
      <div className="space-y-2">
        <Label htmlFor="banned-words">Banned words and phrases, one per line</Label>
        <Textarea
          id="banned-words"
          rows={8}
          value={words}
          onChange={(e) => {
            setWords(e.target.value);
          }}
        />
      </div>
      <p className="text-sm text-muted-foreground">
        Saving checks every {row.name} listing again. An approved listing that now breaks a rule
        loses its approval and needs approving again.
      </p>
      <div className="flex gap-2">
        <Button type="submit" className="h-11" disabled={!limitOk || saving}>
          {saving ? "Saving…" : "Save rules"}
        </Button>
        <Button type="button" variant="outline" className="h-11" onClick={onCancel}>
          Cancel
        </Button>
      </div>
    </form>
  );
}

/**
 * ChannelsView: GET /v1/channels, and PATCH /v1/channels/{channel} for a
 * reviewer. The channel files hold the starting rules; a reviewer's change is
 * recorded with their name and re-checks the channel's listings at once.
 */
export function ChannelsView({
  status,
  channels = [],
  requestId,
  onSave,
  saving = false,
  saved,
  saveError,
}: ChannelsViewProps) {
  const broken = channels.filter((c) => c.loadError);
  const [editingId, setEditingId] = useState<string | undefined>();
  const editing = channels.find((c) => c.id === editingId);
  return (
    <div className="space-y-6">
      <header className="space-y-1">
        <h1 className="text-2xl font-semibold tracking-tight">Channels</h1>
        <p className="max-w-prose text-muted-foreground">
          The rules every listing is checked against.{" "}
          {onSave
            ? "Reviewers can change a channel's rules here; existing listings are checked again at once."
            : "Reviewers can change them; existing listings are then checked again."}
        </p>
      </header>

      {broken.map((c) => (
        <Alert key={c.id} variant="destructive" role="alert">
          <AlertTitle>{c.name} is switched off</AlertTitle>
          <AlertDescription>
            Its rules file could not be read: {c.loadError}. Listings for {c.name} are not written,
            checked or exported until the file is fixed and deployed. Other channels work as normal.
          </AlertDescription>
        </Alert>
      ))}

      {saved ? (
        <Alert role="status">
          <AlertTitle>
            {channels.find((c) => c.id === saved.channel)?.name ?? saved.channel} rules saved
          </AlertTitle>
          <AlertDescription>
            {saved.rechecked} listings checked again; {saved.cleared} approvals cleared because the
            listing now breaks a rule.
          </AlertDescription>
        </Alert>
      ) : null}
      {saveError ? (
        <Alert variant="destructive" role="alert">
          <AlertTitle>The rules were not saved</AlertTitle>
          <AlertDescription>{saveError}</AlertDescription>
        </Alert>
      ) : null}

      {status === "error" ? (
        <Alert variant="destructive" role="alert">
          <AlertTitle>Channels could not be loaded</AlertTitle>
          <AlertDescription>
            Reload the page. If it keeps failing, quote {requestId} to the Catalift team.
          </AlertDescription>
        </Alert>
      ) : status === "loading" ? (
        <div className="space-y-2" aria-busy="true" aria-label="Loading channels">
          <Skeleton className="h-12 w-full" />
          <Skeleton className="h-12 w-full" />
        </div>
      ) : (
        <div className="overflow-x-auto rounded-xl border bg-card shadow-sm">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Channel</TableHead>
                <TableHead>State</TableHead>
                <TableHead className="text-right">Title limit</TableHead>
                <TableHead>Required attributes</TableHead>
                <TableHead className="text-right">Banned words</TableHead>
                <TableHead>Last re-check</TableHead>
                {onSave ? (
                  <TableHead>
                    <span className="sr-only">Edit</span>
                  </TableHead>
                ) : null}
              </TableRow>
            </TableHeader>
            <TableBody>
              {channels.map((c) => (
                <TableRow key={c.id}>
                  <TableCell className="font-medium">{c.name}</TableCell>
                  <TableCell>
                    {c.enabled ? (
                      <Badge variant="secondary">On</Badge>
                    ) : (
                      <Badge variant="destructive">Off: file error</Badge>
                    )}
                  </TableCell>
                  <TableCell className="text-right tabular-nums">
                    {c.titleMaxLength ?? <span className="text-muted-foreground">none</span>}
                  </TableCell>
                  <TableCell>{c.requiredAttributes.join(", ") || "none"}</TableCell>
                  <TableCell className="text-right tabular-nums">{c.bannedWordCount}</TableCell>
                  <TableCell className="text-sm">
                    {c.lastEdit ? (
                      <span className="block">
                        Changed by {c.lastEdit.by}
                        <span className="block text-muted-foreground">{c.lastEdit.when}</span>
                      </span>
                    ) : null}
                    {c.lastRecheck ? (
                      <>
                        {c.lastRecheck.listingsRechecked} checked, {c.lastRecheck.approvalsCleared}{" "}
                        approvals cleared
                        <span className="block text-muted-foreground">{c.lastRecheck.when}</span>
                      </>
                    ) : (
                      <span className="text-muted-foreground">
                        {c.lastEdit ? "" : "Never changed"}
                      </span>
                    )}
                  </TableCell>
                  {onSave ? (
                    <TableCell>
                      {c.enabled && c.configHash ? (
                        <Button
                          variant="outline"
                          size="sm"
                          aria-label={`Edit rules for ${c.name}`}
                          onClick={() => {
                            setEditingId(c.id);
                          }}
                        >
                          <Pencil aria-hidden />
                          Edit rules
                        </Button>
                      ) : null}
                    </TableCell>
                  ) : null}
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}

      {editing && onSave ? (
        <RulesEditor
          key={`${editing.id}:${editing.configHash ?? ""}`}
          row={editing}
          saving={saving}
          onSave={(edit) => {
            onSave(editing, edit);
          }}
          onCancel={() => {
            setEditingId(undefined);
          }}
        />
      ) : null}
    </div>
  );
}
