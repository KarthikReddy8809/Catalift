import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Progress } from "@/components/ui/progress";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

import { PhotoPicker } from "./PhotoPicker";

export interface RowError {
  rowNumber: number;
  sku: string | null;
  reason: string;
}

export interface UploadSummary {
  fileName: string;
  rowsTotal: number;
  rowsAccepted: number;
  rowsRejected: number;
  rowErrors: RowError[];
}

export interface ImagesSummary {
  attached: number;
  unmatchedFiles: string[];
  productsMissingImage: string[];
  /** Which photo went to which SKU, so the seller can check the mapping. */
  attachedFiles?: { fileName: string; sku: string }[] | undefined;
  /** Photos that matched a SKU but were refused (type or size). */
  rejectedFiles?: { fileName: string; reason: string }[] | undefined;
}

export type UploadProblem =
  | { kind: "too-large"; fileName: string }
  | { kind: "wrong-type"; fileName: string }
  | { kind: "conflict" }
  | { kind: "server"; requestId: string };

export interface UploadViewProps {
  step: "csv" | "uploading" | "images";
  uploadingFile?: string;
  progress?: number;
  summary?: UploadSummary;
  images?: ImagesSummary;
  problem?: UploadProblem;
  /** Called with the chosen CSV; absent in the design gallery. */
  onUploadCsv?: (file: File) => void;
  /** Called with the chosen photos. */
  onUploadImages?: (files: File[]) => void;
  /** The photos are on their way. */
  imagesUploading?: boolean;
  /** Leaves for the products screen. */
  onDone?: () => void;
}

/** filesOf reads the chosen files of a form's file input. */
function filesOf(form: HTMLFormElement, name: string): File[] {
  const input = form.elements.namedItem(name);
  return input instanceof HTMLInputElement && input.files ? Array.from(input.files) : [];
}

/** bySku groups attached photos under their SKU, in SKU order. */
function bySku(files: { fileName: string; sku: string }[]): [string, string[]][] {
  const groups = new Map<string, string[]>();
  for (const f of files) groups.set(f.sku, [...(groups.get(f.sku) ?? []), f.fileName]);
  return [...groups.entries()].sort(([a], [b]) => a.localeCompare(b));
}

function problemText(p: UploadProblem): { title: string; body: string } {
  switch (p.kind) {
    case "too-large":
      return {
        title: `${p.fileName} is too large`,
        body: "A CSV can be at most 5 MB and an image at most 10 MB. Split the file and upload the parts.",
      };
    case "wrong-type":
      return {
        title: `${p.fileName} is not a file Catalift can read`,
        body: "Upload the product list as CSV, and photos as JPEG, PNG or WebP.",
      };
    case "conflict":
      return {
        title: "This upload was already sent with different contents",
        body: "Reload the page and upload the file again; nothing from the second attempt was saved.",
      };
    case "server":
      return {
        title: "The upload did not finish",
        body: `Nothing was saved. Try again; if it fails again, quote ${p.requestId} to the Catalift team.`,
      };
  }
}

/**
 * UploadView: POST /v1/uploads (CSV), then POST /v1/uploads/{id}/images.
 * Rows are accepted or rejected one by one (Q-020, D21); images attach by
 * SKU plus a separator, longest SKU first (D20).
 */
export function UploadView({
  step,
  uploadingFile,
  progress,
  summary,
  images,
  problem,
  onUploadCsv,
  onUploadImages,
  imagesUploading = false,
  onDone,
}: UploadViewProps) {
  const p = problem ? problemText(problem) : null;
  return (
    <div className="space-y-6">
      <header className="space-y-1">
        <h1 className="text-2xl font-semibold tracking-tight">Upload a launch</h1>
        <p className="max-w-prose text-muted-foreground">
          First the product list as CSV with columns sku, category, brand and price. Then the folder
          of photos: each file name starts with its SKU, such as <code>KU-104_front.jpg</code> or{" "}
          <code>KU-104-2.png</code>, and is matched to that product.
        </p>
      </header>

      {p ? (
        <Alert variant="destructive" role="alert">
          <AlertTitle>{p.title}</AlertTitle>
          <AlertDescription>{p.body}</AlertDescription>
        </Alert>
      ) : null}

      <div className="grid gap-6 lg:grid-cols-2">
        <section
          aria-labelledby="csv-step"
          className="space-y-3 rounded-xl border bg-card p-5 shadow-sm"
        >
          <h2 id="csv-step" className="font-medium">
            1. Product list
          </h2>
          {step === "uploading" ? (
            <div className="space-y-2" role="status" aria-live="polite">
              <p className="text-sm">Reading {uploadingFile}&hellip;</p>
              <Progress value={progress ?? 0} aria-label="Upload progress" />
            </div>
          ) : summary ? (
            <div className="space-y-3">
              <p className="text-sm">
                <span className="font-medium">{summary.fileName}</span>:{" "}
                <span className="tabular-nums">{summary.rowsAccepted}</span> of{" "}
                <span className="tabular-nums">{summary.rowsTotal}</span> rows became products.
              </p>
              {summary.rowsRejected > 0 ? (
                <div className="space-y-2">
                  <p className="text-sm text-muted-foreground">
                    {summary.rowsRejected} rows were not loaded. Fix them in the CSV and upload only
                    those rows again.
                  </p>
                  <div className="overflow-x-auto">
                    <Table>
                      <TableHeader>
                        <TableRow>
                          <TableHead className="w-16">Row</TableHead>
                          <TableHead>SKU</TableHead>
                          <TableHead>Why it was not loaded</TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {summary.rowErrors.map((e) => (
                          <TableRow key={e.rowNumber}>
                            <TableCell className="tabular-nums">{e.rowNumber}</TableCell>
                            <TableCell>
                              {e.sku ?? <span className="text-muted-foreground">none</span>}
                            </TableCell>
                            <TableCell>{e.reason}</TableCell>
                          </TableRow>
                        ))}
                      </TableBody>
                    </Table>
                  </div>
                </div>
              ) : (
                <Badge variant="secondary">Every row loaded</Badge>
              )}
            </div>
          ) : (
            <form
              className="space-y-2"
              onSubmit={(e) => {
                e.preventDefault();
                const file = filesOf(e.currentTarget, "csv")[0];
                if (file) onUploadCsv?.(file);
              }}
            >
              <Label htmlFor="csv-file">CSV file, up to 5 MB</Label>
              <Input id="csv-file" name="csv" type="file" accept=".csv,text/csv" className="h-11" />
              <Button type="submit" className="h-11">
                Upload product list
              </Button>
            </form>
          )}
        </section>

        <section
          aria-labelledby="img-step"
          className="space-y-3 rounded-xl border bg-card p-5 shadow-sm"
        >
          <h2 id="img-step" className="font-medium">
            2. Photos
          </h2>
          {step === "images" && images ? (
            <div className="space-y-3 text-sm">
              <p>
                <span className="tabular-nums">{images.attached}</span> photos attached to their
                products.
              </p>
              {images.attachedFiles && images.attachedFiles.length > 0 ? (
                <div className="max-h-64 overflow-y-auto rounded-lg border">
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>SKU</TableHead>
                        <TableHead>Photos</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {bySku(images.attachedFiles).map(([sku, files]) => (
                        <TableRow key={sku}>
                          <TableCell className="font-medium">{sku}</TableCell>
                          <TableCell className="whitespace-normal text-muted-foreground">
                            {files.join(", ")}
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </div>
              ) : null}
              {images.unmatchedFiles.length > 0 ? (
                <div className="space-y-1">
                  <p className="font-medium">Not matched to any SKU</p>
                  <p className="text-muted-foreground">
                    Rename each so it starts with its SKU followed by _, - or a dot, then upload it
                    again.
                  </p>
                  <ul className="list-disc pl-5">
                    {images.unmatchedFiles.map((f) => (
                      <li key={f}>{f}</li>
                    ))}
                  </ul>
                </div>
              ) : null}
              {images.rejectedFiles && images.rejectedFiles.length > 0 ? (
                <div className="space-y-1">
                  <p className="font-medium">Photos refused</p>
                  <ul className="list-disc pl-5">
                    {images.rejectedFiles.map((f) => (
                      <li key={f.fileName}>
                        {f.fileName}: {f.reason}
                      </li>
                    ))}
                  </ul>
                </div>
              ) : null}
              {images.productsMissingImage.length > 0 ? (
                <div className="space-y-1">
                  <p className="font-medium">Products still without a photo</p>
                  <p className="text-muted-foreground">
                    These are skipped by detection and generation until a photo is added.
                  </p>
                  <p>{images.productsMissingImage.join(", ")}</p>
                </div>
              ) : null}
              <Button className="h-11" onClick={onDone}>
                Go to products
              </Button>
            </div>
          ) : summary ? (
            <PhotoPicker onUpload={onUploadImages} busy={imagesUploading} />
          ) : (
            <p className="text-sm text-muted-foreground">
              Available once the product list is uploaded, so each photo can find its SKU.
            </p>
          )}
        </section>
      </div>
    </div>
  );
}
