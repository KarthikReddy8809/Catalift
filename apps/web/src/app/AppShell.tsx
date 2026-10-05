import { useQuery, useQueryClient, useSuspenseQuery } from "@tanstack/react-query";
import { Outlet, useNavigate, useRouterState } from "@tanstack/react-router";

import { sessionKeys, sessionQueryOptions, signOut } from "@/features/auth/api";
import { budgetQueryOptions } from "@/features/shell/api";
import { AppFrame, type NavKey } from "@/features/shell/components/AppFrame";
import { useTheme } from "@/lib/theme";

import { SCREEN_PATH } from "./screen-paths";

function navKeyFor(pathname: string): NavKey {
  if (pathname.startsWith("/upload")) return "upload";
  if (pathname.startsWith("/review")) return "review";
  if (pathname.startsWith("/export")) return "export";
  if (pathname.startsWith("/channels")) return "channels";
  return "products";
}

/**
 * AppShell is the layout of every signed-in page: the route's beforeLoad has
 * already made sure there is a session, so the frame always has a user.
 */
export function AppShell() {
  const { data: session } = useSuspenseQuery(sessionQueryOptions());
  const budget = useQuery(budgetQueryOptions());
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const pathname = useRouterState({ select: (s) => s.location.pathname });
  const { theme, toggle } = useTheme();

  if (!session) return null;
  const b = budget.data;
  return (
    <AppFrame
      current={navKeyFor(pathname)}
      role={session.user.role}
      email={session.user.email}
      budget={{
        spentMicroUsd: b?.spent_micro_usd ?? 0,
        limitMicroUsd: b?.limit_micro_usd ?? 0,
        blockedAt: b?.blocked_at ?? null,
      }}
      hrefFor={(screen) => SCREEN_PATH[screen] ?? "/"}
      theme={theme}
      onToggleTheme={toggle}
      onNavigate={(href) => {
        void navigate({ to: href });
      }}
      onSignOut={() => {
        void signOut().finally(() => {
          queryClient.setQueryData(sessionKeys.current, null);
          queryClient.removeQueries({ predicate: (q) => q.queryKey[0] !== "session" });
          void navigate({ to: "/sign-in" });
        });
      }}
    >
      <Outlet />
    </AppFrame>
  );
}
