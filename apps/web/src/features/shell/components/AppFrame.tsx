import { LogOut, Moon, Sparkles, Sun } from "lucide-react";
import type { MouseEvent, ReactNode } from "react";

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Progress } from "@/components/ui/progress";
import { cn } from "@/lib/utils";

import { formatUsdMicro } from "../format";
import { NAV, navAllowed, type NavKey, type Role } from "../nav";

export type { NavKey, Role } from "../nav";

export interface BudgetSummary {
  spentMicroUsd: number;
  limitMicroUsd: number;
  blockedAt: string | null;
}

export interface AppFrameProps {
  current: NavKey;
  role: Role;
  email: string;
  budget: BudgetSummary;
  children: ReactNode;
  /** Where a nav item points. In the design gallery it opens that screen's design. */
  hrefFor?: (screen: string) => string;
  /** In-app navigation for a nav link; without it the link loads the page. */
  onNavigate?: (href: string) => void;
  /** Ends the session; the sign-out control shows only when this is given. */
  onSignOut?: () => void;
  /** The current theme and its switch; the switch shows only when both are given. */
  theme?: "light" | "dark";
  onToggleTheme?: () => void;
}

const galleryHref = (screen: string) => `/__design/${screen}`;

/** Brand mark: the green tile and the wordmark. */
export function BrandMark({ className }: { className?: string }) {
  return (
    <span className={cn("flex items-center gap-2 font-semibold tracking-tight", className)}>
      <span className="grid size-8 place-items-center rounded-lg bg-primary text-primary-foreground shadow-sm">
        <Sparkles className="size-4" aria-hidden />
      </span>
      <span>Catalift</span>
    </span>
  );
}

function initials(email: string): string {
  const name = email.split("@")[0] ?? "";
  const parts = name.split(/[._-]/).filter(Boolean);
  return ((parts[0]?.[0] ?? "") + (parts[1]?.[0] ?? "")).toUpperCase() || "?";
}

/**
 * AppFrame is the chrome every signed-in screen sits in: a sidebar with the
 * navigation by role and the AI spend against its limit (an icon rail on
 * tablets, a bottom tab bar on phones), a header with the page title, the
 * theme switch and the person signed in, then the blocked-budget banner (D13).
 */
export function AppFrame({
  current,
  role,
  email,
  budget,
  children,
  hrefFor = galleryHref,
  onNavigate,
  onSignOut,
  theme,
  onToggleTheme,
}: AppFrameProps) {
  const follow = (e: MouseEvent<HTMLAnchorElement>) => {
    if (!onNavigate || e.metaKey || e.ctrlKey || e.shiftKey || e.button !== 0) return;
    e.preventDefault();
    onNavigate(e.currentTarget.getAttribute("href") ?? "/");
  };
  const items = NAV.filter((n) => navAllowed(role, n.key));
  const spent = formatUsdMicro(budget.spentMicroUsd);
  const limit = formatUsdMicro(budget.limitMicroUsd);
  const share = budget.limitMicroUsd > 0 ? (budget.spentMicroUsd / budget.limitMicroUsd) * 100 : 0;
  const currentLabel = items.find((n) => n.key === current)?.label ?? "";
  return (
    <div className="min-h-svh bg-gradient-to-b from-brand-soft/60 via-background to-background pb-20 md:pb-0">
      <aside className="fixed inset-y-0 left-0 z-30 hidden w-16 flex-col border-r bg-sidebar text-sidebar-foreground md:flex lg:w-64">
        <a
          href={hrefFor("S-04")}
          onClick={follow}
          aria-label="Catalift home"
          className="flex h-16 items-center justify-center border-b px-3 lg:justify-start lg:px-5"
        >
          <BrandMark className="[&>span:last-child]:hidden lg:[&>span:last-child]:inline" />
        </a>

        <nav aria-label="Catalift" className="flex-1 px-2 py-4 lg:px-3">
          <p className="mb-2 hidden px-3 text-xs font-medium tracking-wide text-muted-foreground uppercase lg:block">
            Launch
          </p>
          <ul className="space-y-1">
            {items.map((n) => (
              <li key={n.key}>
                <a
                  href={hrefFor(n.screen)}
                  onClick={follow}
                  title={n.label}
                  aria-current={n.key === current ? "page" : undefined}
                  className={cn(
                    "flex h-10 items-center justify-center gap-3 rounded-lg px-3 text-sm font-medium transition-colors focus-visible:ring-2 focus-visible:ring-sidebar-ring focus-visible:outline-none lg:justify-start",
                    n.key === current
                      ? "bg-sidebar-primary text-sidebar-primary-foreground shadow-sm"
                      : "text-muted-foreground hover:bg-sidebar-accent hover:text-sidebar-accent-foreground",
                  )}
                >
                  <n.icon className="size-4 shrink-0" aria-hidden />
                  <span className="sr-only lg:not-sr-only">{n.label}</span>
                </a>
              </li>
            ))}
          </ul>
        </nav>

        <div className="hidden border-t p-4 lg:block">
          <div className="space-y-2 rounded-lg bg-sidebar-accent/60 p-3">
            <p className="flex justify-between text-xs text-muted-foreground tabular-nums">
              <span>AI spend</span>
              <span>
                <span className="font-medium text-foreground">{spent}</span> of {limit}
              </span>
            </p>
            <Progress value={Math.min(share, 100)} aria-label="AI spend against the limit" />
          </div>
        </div>
      </aside>

      <div className="md:pl-16 lg:pl-64">
        <header className="sticky top-0 z-20 border-b bg-background/80 backdrop-blur supports-[backdrop-filter]:bg-background/70">
          <div className="mx-auto flex h-16 max-w-6xl items-center gap-4 px-4 lg:px-8">
            <a
              href={hrefFor("S-04")}
              onClick={follow}
              aria-label="Catalift home"
              className="md:hidden"
            >
              <BrandMark />
            </a>
            <p className="hidden text-sm font-medium text-muted-foreground md:block">
              {currentLabel}
            </p>

            <div className="ml-auto flex items-center gap-3">
              <p className="hidden text-xs text-muted-foreground tabular-nums sm:block lg:hidden">
                AI spend <span className="font-medium text-foreground">{spent}</span> of {limit}
              </p>

              {theme && onToggleTheme ? (
                <Button
                  variant="ghost"
                  size="icon"
                  className="rounded-full"
                  onClick={onToggleTheme}
                  aria-label={theme === "dark" ? "Use the light theme" : "Use the dark theme"}
                >
                  {theme === "dark" ? <Sun aria-hidden /> : <Moon aria-hidden />}
                </Button>
              ) : null}

              <div className="flex items-center gap-2 border-l pl-3">
                <Avatar className="size-8" title={`${email} (${role})`}>
                  <AvatarFallback className="bg-secondary text-xs font-semibold text-secondary-foreground">
                    {initials(email)}
                  </AvatarFallback>
                </Avatar>
                <div className="hidden leading-tight sm:block">
                  <p className="max-w-48 truncate text-sm font-medium">{email}</p>
                  <Badge variant="outline" className="mt-0.5 h-5 px-1.5 text-[11px]">
                    {role === "reviewer" ? "Reviewer" : "Seller"}
                  </Badge>
                </div>
                {onSignOut ? (
                  <Button
                    variant="ghost"
                    size="icon"
                    className="rounded-full"
                    onClick={onSignOut}
                    aria-label="Sign out"
                    title="Sign out"
                  >
                    <LogOut aria-hidden />
                  </Button>
                ) : null}
              </div>
            </div>
          </div>
        </header>

        <main className="mx-auto max-w-6xl space-y-6 px-4 py-8 lg:px-8">
          {budget.blockedAt ? (
            <Alert variant="destructive">
              <AlertTitle>AI calls are blocked</AlertTitle>
              <AlertDescription>
                Spend reached {spent} of the {limit} limit, so detection, generation and
                regeneration are paused. Work already done is kept. The owner can raise the limit
                and clear the block; then resume stopped work from Products.
              </AlertDescription>
            </Alert>
          ) : null}

          {children}
        </main>
      </div>

      <nav
        aria-label="Catalift on a phone"
        className="fixed inset-x-0 bottom-0 z-20 border-t bg-background/90 backdrop-blur md:hidden"
      >
        <ul className="grid" style={{ gridTemplateColumns: `repeat(${items.length}, 1fr)` }}>
          {items.map((n) => (
            <li key={n.key}>
              <a
                href={hrefFor(n.screen)}
                onClick={follow}
                aria-current={n.key === current ? "page" : undefined}
                className={cn(
                  "flex h-16 flex-col items-center justify-center gap-1 text-xs focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none",
                  n.key === current ? "font-semibold text-primary" : "text-muted-foreground",
                )}
              >
                <n.icon className="size-5" aria-hidden />
                {n.label}
              </a>
            </li>
          ))}
        </ul>
      </nav>
    </div>
  );
}
