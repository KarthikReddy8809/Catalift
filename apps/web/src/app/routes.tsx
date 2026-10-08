import type { QueryClient } from "@tanstack/react-query";
import {
  createRootRouteWithContext,
  createRoute,
  createRouter,
  redirect,
  type RouterHistory,
} from "@tanstack/react-router";
import { z } from "zod";

import { AppShell } from "@/app/AppShell";
import { NotFound } from "@/app/NotFound";
import { RootLayout } from "@/app/RootLayout";
import { RouteError } from "@/app/RouteError";
import { GalleryIndex, GalleryScreen } from "@/design/Gallery";
import { parseGallerySearch } from "@/design/registry";
import { sessionQueryOptions } from "@/features/auth/api";
import { SignInPage } from "@/features/auth/pages/SignInPage";
import { BrandVoicePage } from "@/features/brands/pages/BrandVoicePage";
import { ProductsPage } from "@/features/catalogue/pages/ProductsPage";
import { UploadPage } from "@/features/catalogue/pages/UploadPage";
import { ChannelsPage } from "@/features/channels/pages/ChannelsPage";
import { ExportPage } from "@/features/exports/pages/ExportPage";
import { ReceivedPage } from "@/features/exports/pages/ReceivedPage";
import { ReviewPage } from "@/features/listings/pages/ReviewPage";
import { navAllowed, type NavKey } from "@/features/shell/nav";

// Code-based routes. A route that needs data adds
// `loader: ({ context }) => context.queryClient.ensureQueryData(options)`
// and its component calls `useSuspenseQuery(options)`; a route that reads the
// URL adds `validateSearch: searchSchema.parse`. A loader or component that
// throws lands in RouteError (the root errorComponent) with the layout intact.
interface RouterContext {
  queryClient: QueryClient;
}

const rootRoute = createRootRouteWithContext<RouterContext>()({
  component: RootLayout,
  errorComponent: RouteError,
  notFoundComponent: NotFound,
});

// "/" sends a signed-in person to Products and everyone else to sign-in.
const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/",
  beforeLoad: async ({ context }) => {
    const session = await context.queryClient.query(sessionQueryOptions());
    // TanStack Router's redirect is thrown by design.
    // eslint-disable-next-line @typescript-eslint/only-throw-error
    throw redirect({ to: session ? "/products" : "/sign-in" });
  },
});

const signInRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/sign-in",
  validateSearch: (s: Record<string, unknown>) =>
    z.object({ expired: z.boolean().optional() }).parse(s),
  component: function SignInRoute() {
    const { expired } = signInRoute.useSearch();
    return <SignInPage expired={expired === true} />;
  },
});

// Every signed-in page sits in the app frame; no session goes to sign-in.
const appRoute = createRoute({
  getParentRoute: () => rootRoute,
  id: "app",
  beforeLoad: async ({ context }) => {
    const session = await context.queryClient.query(sessionQueryOptions());
    if (!session) {
      // eslint-disable-next-line @typescript-eslint/only-throw-error
      throw redirect({ to: "/sign-in" });
    }
  },
  component: AppShell,
});

/**
 * onlyFor sends a role to Products when it opens a page that is not in its
 * navigation (ADR-0012), such as a seller typing /review. The API refuses
 * the actions anyway; this keeps the person on a page they can use.
 */
const onlyFor =
  (key: NavKey) =>
  async ({ context }: { context: { queryClient: QueryClient } }) => {
    const session = await context.queryClient.query(sessionQueryOptions());
    if (session && !navAllowed(session.user.role, key)) {
      // eslint-disable-next-line @typescript-eslint/only-throw-error
      throw redirect({ to: "/products" });
    }
  };

const uploadRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/upload",
  beforeLoad: onlyFor("upload"),
  component: UploadPage,
});

const brandsRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/brands",
  beforeLoad: onlyFor("brands"),
  component: BrandVoicePage,
});

const receivedRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/received",
  beforeLoad: onlyFor("received"),
  component: ReceivedPage,
});

const productsRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/products",
  validateSearch: (s: Record<string, unknown>) =>
    z.object({ upload: z.coerce.string().optional() }).parse(s),
  component: function ProductsRoute() {
    const { upload } = productsRoute.useSearch();
    return <ProductsPage upload={upload} />;
  },
});

const reviewRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/review",
  beforeLoad: onlyFor("review"),
  component: ReviewPage,
});

const exportRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/export",
  beforeLoad: onlyFor("export"),
  component: ExportPage,
});

const channelsRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/channels",
  beforeLoad: onlyFor("channels"),
  component: ChannelsPage,
});

const appTree = appRoute.addChildren([
  uploadRoute,
  productsRoute,
  brandsRoute,
  receivedRoute,
  reviewRoute,
  exportRoute,
  channelsRoute,
]);

// The design gallery: every *.screen.tsx in every state, inside this layout.
// On in `vite dev`, and in a build only with VITE_DESIGN_GALLERY=1 (the
// design review and the smoke run use it); a production build drops it.
export const designGalleryEnabled =
  import.meta.env.DEV || import.meta.env.VITE_DESIGN_GALLERY === "1";

const galleryIndexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/__design",
  component: () => (
    <main className="mx-auto max-w-6xl px-4 py-8">
      <GalleryIndex />
    </main>
  ),
});

const galleryScreenRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/__design/$id",
  validateSearch: parseGallerySearch,
  component: function GalleryScreenRoute() {
    const { id } = galleryScreenRoute.useParams();
    const search = galleryScreenRoute.useSearch();
    // Screens bring their own frame and padding; only the state switcher needs a gutter.
    return (
      <div className="[&>div>[role=tablist]]:m-4">
        <GalleryScreen id={id} search={search} />
      </div>
    );
  },
});

export const routeTree = rootRoute.addChildren(
  designGalleryEnabled
    ? [indexRoute, signInRoute, appTree, galleryIndexRoute, galleryScreenRoute]
    : [indexRoute, signInRoute, appTree],
);

interface AppRouterOptions {
  queryClient: QueryClient;
  /** Memory history for tests; browser history by default. */
  history?: RouterHistory;
}

export function createAppRouter({ queryClient, history }: AppRouterOptions) {
  return createRouter({
    routeTree,
    context: { queryClient },
    defaultPreload: "intent",
    defaultErrorComponent: RouteError,
    scrollRestoration: true,
    ...(history ? { history } : {}),
  });
}

declare module "@tanstack/react-router" {
  interface Register {
    router: ReturnType<typeof createAppRouter>;
  }
}
