import path from "node:path";

import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

// Vite build and dev server. Test settings live in vitest.config.ts.
export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: { "@": path.resolve(import.meta.dirname, "src") },
  },
  // The dev proxy stands in for the production reverse proxy (HLD section 3):
  // web app and API share one origin, so the API needs no CORS.
  server: {
    port: 5173,
    strictPort: true,
    proxy: Object.fromEntries(
      ["/v1", "/healthz", "/readyz"].map((p) => [p, "http://localhost:8080"]),
    ),
  },
  preview: { port: 4173, strictPort: true },
  build: {
    sourcemap: true,
    // A chunk over this size is a review question, not a warning to silence.
    chunkSizeWarningLimit: 500,
  },
});
