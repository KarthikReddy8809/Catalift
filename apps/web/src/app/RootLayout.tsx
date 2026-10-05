import { Outlet } from "@tanstack/react-router";
import { Suspense } from "react";

/**
 * RootLayout frames every route. Signed-in pages bring their own header and
 * navigation (AppFrame); sign-in and the design gallery lay themselves out.
 * The Suspense boundary is where a lazy route or a useSuspenseQuery waits.
 */
export function RootLayout() {
  return (
    <div className="min-h-svh bg-background text-foreground">
      <Suspense
        fallback={
          <p role="status" aria-live="polite" className="p-8 text-muted-foreground">
            Loading
          </p>
        }
      >
        <Outlet />
      </Suspense>
    </div>
  );
}
