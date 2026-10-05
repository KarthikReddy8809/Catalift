import { useCallback, useEffect, useState } from "react";

// Light or dark, remembered per browser. The first choice follows the
// system setting; index.html applies the stored one before React paints, so
// a reload does not flash the other theme. Storage may be unavailable; the
// theme then simply follows the system each visit.

export type Theme = "light" | "dark";

const KEY = "catalift.theme";

export function storedTheme(): Theme {
  try {
    const v = localStorage.getItem(KEY);
    if (v === "light" || v === "dark") return v;
  } catch {
    // Storage blocked: fall through to the system setting.
  }
  const prefersDark =
    typeof window.matchMedia === "function" &&
    window.matchMedia("(prefers-color-scheme: dark)").matches;
  return prefersDark ? "dark" : "light";
}

export function applyTheme(theme: Theme): void {
  document.documentElement.classList.toggle("dark", theme === "dark");
  document.documentElement.style.colorScheme = theme;
}

/** useTheme returns the current theme and a toggle that applies and remembers it. */
export function useTheme(): { theme: Theme; toggle: () => void } {
  const [theme, setTheme] = useState<Theme>(storedTheme);
  useEffect(() => {
    applyTheme(theme);
  }, [theme]);
  const toggle = useCallback(() => {
    setTheme((t) => {
      const next = t === "dark" ? "light" : "dark";
      try {
        localStorage.setItem(KEY, next);
      } catch {
        // Not remembered; it still applies for this visit.
      }
      return next;
    });
  }, []);
  return { theme, toggle };
}
