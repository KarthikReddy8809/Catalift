import { ImagePlus } from "lucide-react";
import { useState } from "react";

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Sheet, SheetContent, SheetDescription, SheetTitle } from "@/components/ui/sheet";

export interface FixValues {
  sku: string;
  category: string;
  brand: string;
  price: string;
}

/** What the drawer is fixing: a CSV row the upload refused, or a loaded product. */
export interface FixTarget {
  /** "row:<upload>:<row>" or "product:<id>"; changes reset the form. */
  key: string;
  title: string;
  /** Why it needs fixing, as the seller should read it. */
  problem: string;
  values: FixValues;
  /** Photos the product has now; 0 for a rejected row. */
  photoCount: number;
}

/** The result of the last Validate, shown in the drawer. */
export interface FixOutcome {
  tone: "error" | "success";
  message: string;
}

export interface ProductFixDrawerProps {
  target?: FixTarget | undefined;
  categories: string[];
  busy?: boolean | undefined;
  outcome?: FixOutcome | undefined;
  onValidate?: ((values: FixValues, photos: File[]) => void) | undefined;
  /** Drops a rejected row instead of loading it; only offered for rows. */
  onDiscard?: (() => void) | undefined;
  onClose: () => void;
}

const IMAGE = /\.(jpe?g|png|webp)$/i;

function FixForm({
  target,
  categories,
  busy,
  outcome,
  onValidate,
  onDiscard,
}: Omit<ProductFixDrawerProps, "target" | "onClose"> & { target: FixTarget }) {
  const [v, setV] = useState<FixValues>(target.values);
  const [photos, setPhotos] = useState<File[]>([]);
  const set = (f: keyof FixValues) => (e: { target: { value: string } }) => {
    setV((cur) => ({ ...cur, [f]: e.target.value }));
  };
  // A category the file used that is not on the list stays visible, so the
  // seller sees what was typed and picks a known one.
  const options =
    categories.includes(v.category) || !v.category ? categories : [v.category, ...categories];
  const willHavePhoto = target.photoCount + photos.length > 0;
  return (
    <form
      className="flex min-h-full flex-col"
      onSubmit={(e) => {
        e.preventDefault();
        onValidate?.(v, photos);
      }}
    >
      <div className="flex-1 space-y-5 p-5">
        <div className="space-y-1">
          <SheetTitle>{target.title}</SheetTitle>
          <SheetDescription className="text-destructive">{target.problem}</SheetDescription>
        </div>

        {outcome ? (
          <Alert
            variant={outcome.tone === "error" ? "destructive" : "default"}
            role={outcome.tone === "error" ? "alert" : "status"}
          >
            <AlertTitle>{outcome.tone === "error" ? "Not fixed yet" : "Fixed"}</AlertTitle>
            <AlertDescription>{outcome.message}</AlertDescription>
          </Alert>
        ) : null}

        <div className="space-y-2">
          <Label htmlFor="fix-sku">SKU</Label>
          <Input id="fix-sku" value={v.sku} onChange={set("sku")} className="h-11" />
        </div>
        <div className="space-y-2">
          <Label htmlFor="fix-category">Category</Label>
          <Select
            value={v.category}
            onValueChange={(c) => {
              setV((cur) => ({ ...cur, category: c }));
            }}
          >
            <SelectTrigger id="fix-category" className="h-11 w-full">
              <SelectValue placeholder="Choose a category" />
            </SelectTrigger>
            <SelectContent>
              {options.map((c) => (
                <SelectItem key={c} value={c}>
                  {c}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className="space-y-2">
          <Label htmlFor="fix-brand">Brand</Label>
          <Input id="fix-brand" value={v.brand} onChange={set("brand")} className="h-11" />
        </div>
        <div className="space-y-2">
          <Label htmlFor="fix-price">Price (₹)</Label>
          <Input
            id="fix-price"
            value={v.price}
            inputMode="decimal"
            onChange={set("price")}
            className="h-11"
          />
        </div>

        <div className="space-y-2">
          <p className="text-sm font-medium">Photos</p>
          <p className="text-sm text-muted-foreground">
            {target.photoCount > 0
              ? `${String(target.photoCount)} photo${target.photoCount === 1 ? "" : "s"} on file.`
              : "No photo yet; the AI needs one to read the garment."}
            {photos.length > 0
              ? ` ${String(photos.length)} new: ${photos.map((f) => f.name).join(", ")}.`
              : ""}
          </p>
          <Button asChild variant="outline" className="h-11">
            <label htmlFor="fix-photos" className="cursor-pointer">
              <ImagePlus aria-hidden />
              {photos.length > 0 ? "Choose other photos" : "Add photos"}
            </label>
          </Button>
          <input
            id="fix-photos"
            type="file"
            multiple
            accept="image/jpeg,image/png,image/webp"
            className="sr-only"
            aria-label="Photos for this product"
            onChange={(e) => {
              setPhotos(Array.from(e.currentTarget.files ?? []).filter((f) => IMAGE.test(f.name)));
            }}
          />
        </div>
      </div>

      <div className="sticky bottom-0 space-y-2 border-t bg-background p-5">
        <Button type="submit" className="h-11 w-full" disabled={busy}>
          {busy ? "Validating…" : "Validate"}
        </Button>
        {onDiscard ? (
          <Button
            type="button"
            variant="ghost"
            className="h-10 w-full text-destructive"
            disabled={busy}
            onClick={onDiscard}
          >
            Discard this row
          </Button>
        ) : null}
        <p className="text-xs text-muted-foreground">
          {willHavePhoto
            ? "Checks the details with the upload's rules, saves them, uploads the photos and runs the AI on this product again (one vision call)."
            : "Checks the details with the upload's rules and saves them. Add a photo so the AI can run."}
        </p>
      </div>
    </form>
  );
}

/**
 * ProductFixDrawer: a record that failed validation opens here (seller flow
 * step 2). Every field can be corrected and photos added; Validate re-checks
 * with the upload's rules and, once the product has a photo, runs the one
 * vision call again for this product only.
 */
export function ProductFixDrawer({ target, onClose, ...rest }: ProductFixDrawerProps) {
  return (
    <Sheet
      open={target !== undefined}
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <SheetContent side="right" className="w-full gap-0 overflow-y-auto p-0 sm:max-w-md">
        {target ? <FixForm key={target.key} target={target} {...rest} /> : null}
      </SheetContent>
    </Sheet>
  );
}
