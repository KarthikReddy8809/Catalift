// The seller's current launch (upload and run), remembered in this browser so
// closing the page and coming back shows the same progress (AC-US-00-003-6).
// A per-viewer convenience: storage may be empty or unavailable, and the
// page then shows every product with no run.

export interface Launch {
  uploadId?: string | undefined;
  fileName?: string | undefined;
  runId?: string | undefined;
}

const KEY = "catalift.launch";

export function readLaunch(): Launch {
  try {
    const raw = localStorage.getItem(KEY);
    return raw ? (JSON.parse(raw) as Launch) : {};
  } catch {
    return {};
  }
}

export function rememberLaunch(next: Launch): void {
  try {
    localStorage.setItem(KEY, JSON.stringify(next));
  } catch {
    // Storage blocked: the page still works, it just forgets on reload.
  }
}
