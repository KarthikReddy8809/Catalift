import { FolderOpen, ImagePlus, Images, X } from "lucide-react";
import { useState, type DragEvent } from "react";

import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

/** The photo types the API accepts (US-00-001, T-15). */
const IMAGE_EXT = /\.(jpe?g|png|webp)$/i;

function isPhoto(f: File): boolean {
  return IMAGE_EXT.test(f.name) && !f.name.startsWith(".");
}

/** The folder a picked or dropped file came from, if the browser says. */
function folderOf(files: File[]): string | undefined {
  const path = files.find((f) => f.webkitRelativePath)?.webkitRelativePath;
  return path?.split("/")[0];
}

// A dropped folder arrives as a directory entry; walk it to its files.
type Entry = FileSystemEntry;

function readAll(dir: FileSystemDirectoryEntry): Promise<Entry[]> {
  const reader = dir.createReader();
  const out: Entry[] = [];
  return new Promise((resolve, reject) => {
    const next = () => {
      reader.readEntries((batch) => {
        if (batch.length === 0) {
          resolve(out);
          return;
        }
        out.push(...batch);
        next();
      }, reject);
    };
    next();
  });
}

async function filesFrom(entry: Entry): Promise<File[]> {
  if (entry.isFile) {
    return [
      await new Promise<File>((resolve, reject) => {
        (entry as FileSystemFileEntry).file(resolve, reject);
      }),
    ];
  }
  const children = await readAll(entry as FileSystemDirectoryEntry);
  return (await Promise.all(children.map(filesFrom))).flat();
}

async function droppedFiles(
  e: DragEvent<HTMLElement>,
): Promise<{ files: File[]; folder?: string }> {
  const entries = Array.from(e.dataTransfer.items)
    .map((i) => i.webkitGetAsEntry())
    .filter((x): x is Entry => !!x);
  if (entries.length === 0) return { files: Array.from(e.dataTransfer.files) };
  const files = (await Promise.all(entries.map(filesFrom))).flat();
  const dir = entries.find((x) => x.isDirectory);
  return dir ? { files, folder: dir.name } : { files };
}

export interface PhotoPickerProps {
  /** Called with the chosen photos, non-images already left out. */
  onUpload?: ((files: File[]) => void) | undefined;
  busy?: boolean | undefined;
}

/**
 * PhotoPicker: choose a whole folder of photos (subfolders included), pick
 * files, or drop either. Each photo is matched to its SKU by its file name
 * on upload, so the folder can hold every product's photos at once.
 */
export function PhotoPicker({ onUpload, busy = false }: PhotoPickerProps) {
  const [photos, setPhotos] = useState<File[]>([]);
  const [skipped, setSkipped] = useState(0);
  const [folder, setFolder] = useState<string | undefined>();
  const [over, setOver] = useState(false);

  const take = (all: File[], from?: string) => {
    const ok = all.filter(isPhoto);
    setPhotos(ok);
    setSkipped(all.length - ok.length);
    setFolder(from ?? folderOf(all));
  };

  return (
    <div className="space-y-3">
      <div
        role="group"
        aria-label="Drop photos here"
        onDragOver={(e) => {
          e.preventDefault();
          setOver(true);
        }}
        onDragLeave={() => {
          setOver(false);
        }}
        onDrop={(e) => {
          e.preventDefault();
          setOver(false);
          void droppedFiles(e).then(({ files, folder: f }) => {
            take(files, f);
          });
        }}
        className={cn(
          "flex flex-col items-center gap-3 rounded-lg border-2 border-dashed p-6 text-center transition-colors",
          over ? "border-primary bg-accent" : "border-border",
        )}
      >
        <Images className="size-8 text-brand" aria-hidden />
        <p className="text-sm">
          Drop the photos folder here, or choose it. Every photo inside is matched to its SKU by its
          file name.
        </p>
        <div className="flex flex-wrap justify-center gap-2">
          <Button asChild className="h-11">
            <label htmlFor="img-folder" className="cursor-pointer">
              <FolderOpen aria-hidden />
              Choose folder
            </label>
          </Button>
          <Button asChild variant="outline" className="h-11">
            <label htmlFor="img-files" className="cursor-pointer">
              <ImagePlus aria-hidden />
              Choose photos
            </label>
          </Button>
        </div>
        <input
          id="img-folder"
          type="file"
          multiple
          className="sr-only"
          aria-label="Photos folder"
          ref={(el) => {
            el?.setAttribute("webkitdirectory", "");
          }}
          onChange={(e) => {
            take(Array.from(e.currentTarget.files ?? []));
          }}
        />
        <input
          id="img-files"
          type="file"
          multiple
          accept="image/jpeg,image/png,image/webp"
          className="sr-only"
          aria-label="Photos"
          onChange={(e) => {
            take(Array.from(e.currentTarget.files ?? []));
          }}
        />
        <p className="text-xs text-muted-foreground">JPEG, PNG or WebP, up to 10 MB each.</p>
      </div>

      {photos.length > 0 || skipped > 0 ? (
        <div className="space-y-2 rounded-lg bg-muted p-3 text-sm" role="status">
          <div className="flex items-start justify-between gap-2">
            <p>
              <span className="font-medium tabular-nums">{photos.length}</span> photos ready
              {folder ? (
                <>
                  {" "}
                  from <span className="font-medium">{folder}</span>
                </>
              ) : null}
              {skipped > 0 ? (
                <span className="text-muted-foreground">
                  {" "}
                  &middot; {skipped} other files left out
                </span>
              ) : null}
            </p>
            <Button
              variant="ghost"
              size="icon"
              className="size-7"
              aria-label="Clear the chosen photos"
              onClick={() => {
                take([]);
              }}
            >
              <X aria-hidden />
            </Button>
          </div>
          {photos.length > 0 ? (
            <p className="truncate text-muted-foreground">
              {photos
                .slice(0, 6)
                .map((f) => f.name)
                .join(", ")}
              {photos.length > 6 ? `, and ${String(photos.length - 6)} more` : ""}
            </p>
          ) : null}
        </div>
      ) : null}

      <Button
        className="h-11"
        disabled={photos.length === 0 || busy}
        onClick={() => onUpload?.(photos)}
      >
        {busy ? "Uploading photos…" : `Upload ${String(photos.length)} photos`}
      </Button>
    </div>
  );
}
