import type { MouseEvent, ReactNode } from "react";

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

import { formatUsdMicro } from "../format";

export type Role = "seller" | "reviewer";
export type NavKey = "upload" | "products" | "review" | "export" | "channels";

interface NavItem {
  key: NavKey;
  label: string;
  screen: string;
  reviewerOnly?: boolean;
}

// Proposed navigation (no flows file yet): the launch runs left to right,
// upload, products, review, export. Channels is reference, so it sits last.
const NAV: NavItem[] = [
  { key: "upload", label: "Upload", screen: "S-03" },
  { key: "products", label: "Products", screen: "S-04" },
  { key: "review", label: "Review", screen: "S-05" },
  { key: "export", label: "Export", screen: "S-06", reviewerOnly: true },
  { key: "channels", label: "Channels", screen: "S-07" },
];

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
}

const galleryHref = (screen: string) => `/__design/${screen}`;

/**
 * AppFrame is the chrome every signed-in screen sits in: navigation by role,
 * the AI spend against its limit, and the blocked-budget banner (D13).
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
}: AppFrameProps) {
  const follow = (e: MouseEvent<HTMLAnchorElement>) => {
    if (!onNavigate || e.metaKey || e.ctrlKey || e.shiftKey || e.button !== 0) return;
    e.preventDefault();
    onNavigate(e.currentTarget.getAttribute("href") ?? "/");
  };
  const items = NAV.filter((n) => role === "reviewer" || !n.reviewerOnly);
  const spent = formatUsdMicro(budget.spentMicroUsd);
  const limit = formatUsdMicro(budget.limitMicroUsd);
  return (
    <div className="space-y-6 pb-20 md:pb-0">
      <div className="flex flex-wrap items-center justify-between gap-3 border-b pb-3">
        <nav aria-label="Catalift" className="hidden md:block">
          <ul className="flex gap-1">
            {items.map((n) => (
              <li key={n.key}>
                <a
                  href={hrefFor(n.screen)}
                  onClick={follow}
                  aria-current={n.key === current ? "page" : undefined}
                  className={cn(
                    "inline-flex h-9 items-center rounded-md px-3 text-sm font-medium focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:outline-none",
                    n.key === current
                      ? "bg-primary text-primary-foreground"
                      : "text-muted-foreground hover:bg-accent hover:text-accent-foreground",
                  )}
                >
                  {n.label}
                </a>
              </li>
            ))}
          </ul>
        </nav>
        <p className="text-sm text-muted-foreground tabular-nums">
          AI spend <span className="font-medium text-foreground">{spent}</span> of {limit}
        </p>
        <p className="flex items-center gap-2 text-sm">
          <span className="truncate text-muted-foreground">{email}</span>
          <Badge variant="outline">{role === "reviewer" ? "Reviewer" : "Seller"}</Badge>
          {onSignOut ? (
            <Button variant="ghost" size="sm" onClick={onSignOut}>
              Sign out
            </Button>
          ) : null}
        </p>
      </div>

      {budget.blockedAt ? (
        <Alert variant="destructive">
          <AlertTitle>AI calls are blocked</AlertTitle>
          <AlertDescription>
            Spend reached {spent} of the {limit} limit, so detection, generation and regeneration
            are paused. Work already done is kept. The owner can raise the limit and clear the
            block; then resume stopped work from Products.
          </AlertDescription>
        </Alert>
      ) : null}

      {children}

      <nav
        aria-label="Catalift on a phone"
        className="fixed inset-x-0 bottom-0 z-10 border-t bg-background md:hidden"
      >
        <ul className="grid" style={{ gridTemplateColumns: `repeat(${items.length}, 1fr)` }}>
          {items.map((n) => (
            <li key={n.key}>
              <a
                href={hrefFor(n.screen)}
                onClick={follow}
                aria-current={n.key === current ? "page" : undefined}
                className={cn(
                  "flex h-14 items-center justify-center text-sm focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none",
                  n.key === current ? "font-semibold text-foreground" : "text-muted-foreground",
                )}
              >
                {n.label}
              </a>
            </li>
          ))}
        </ul>
      </nav>
    </div>
  );
}
