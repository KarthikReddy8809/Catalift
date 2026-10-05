/* global localStorage, window, document */
// Applies the remembered light or dark theme before the first paint, so a
// reload does not flash the other theme. A file, not inline, because the
// production CSP allows scripts from 'self' only. Mirrors src/lib/theme.ts.
(function () {
  try {
    var t = localStorage.getItem("catalift.theme");
    if (t !== "light" && t !== "dark") {
      t = window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
    }
    if (t === "dark") document.documentElement.classList.add("dark");
  } catch {
    // Storage blocked: the app falls back to the system theme.
  }
})();
