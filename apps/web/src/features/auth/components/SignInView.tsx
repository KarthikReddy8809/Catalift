import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

/** textOf reads a text field of a submitted form. */
function textOf(form: FormData, name: string): string {
  const v = form.get(name);
  return typeof v === "string" ? v : "";
}

export type SignInStatus = "idle" | "submitting" | "wrong-credentials" | "rate-limited" | "expired";

export interface SignInViewProps {
  status: SignInStatus;
  email?: string;
  /** Seconds left before another attempt is allowed (429 Retry-After). */
  retryAfterSeconds?: number;
  /** Called with the typed credentials; absent in the design gallery. */
  onSubmit?: (email: string, password: string) => void;
}

/**
 * SignInView: POST /v1/sessions. No app chrome by design: there is no session
 * yet, so there is nothing to navigate to. Accounts are seeded (ADR-0006), so
 * there is no sign-up or password reset link.
 */
export function SignInView({ status, email = "", retryAfterSeconds, onSubmit }: SignInViewProps) {
  const busy = status === "submitting";
  const locked = status === "rate-limited";
  return (
    <div className="mx-auto grid max-w-4xl gap-8 md:grid-cols-[1fr_minmax(0,24rem)] md:items-center">
      <section aria-labelledby="pitch" className="space-y-3">
        <h1 id="pitch" className="text-3xl font-semibold tracking-tight text-balance">
          Catalift
        </h1>
        <p className="max-w-prose text-muted-foreground">
          Product photos in, listings for every channel out: written by AI, checked against each
          channel&rsquo;s rules, and approved by a reviewer before anything is exported.
        </p>
      </section>

      <Card>
        <CardHeader>
          <CardTitle>Sign in</CardTitle>
          <CardDescription>Use the account the Catalift team set up for you.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {status === "wrong-credentials" ? (
            <Alert variant="destructive" role="alert">
              <AlertTitle>Email or password is wrong</AlertTitle>
              <AlertDescription>Check both and try again.</AlertDescription>
            </Alert>
          ) : null}
          {locked ? (
            <Alert variant="destructive" role="alert">
              <AlertTitle>Too many attempts</AlertTitle>
              <AlertDescription>
                Sign-in from this network is paused for {retryAfterSeconds ?? 0} seconds. Wait, then
                try again.
              </AlertDescription>
            </Alert>
          ) : null}
          {status === "expired" ? (
            <Alert role="status">
              <AlertTitle>Your session ended</AlertTitle>
              <AlertDescription>Sign in again to carry on where you were.</AlertDescription>
            </Alert>
          ) : null}

          <form
            className="space-y-4"
            aria-busy={busy}
            onSubmit={(e) => {
              e.preventDefault();
              const data = new FormData(e.currentTarget);
              onSubmit?.(textOf(data, "email"), textOf(data, "password"));
            }}
          >
            <div className="space-y-2">
              <Label htmlFor="email">Email</Label>
              <Input
                id="email"
                name="email"
                type="email"
                required
                autoComplete="username"
                defaultValue={email}
                aria-invalid={status === "wrong-credentials"}
                disabled={busy || locked}
                className="h-11"
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="password">Password</Label>
              <Input
                id="password"
                name="password"
                type="password"
                required
                autoComplete="current-password"
                aria-invalid={status === "wrong-credentials"}
                disabled={busy || locked}
                className="h-11"
              />
            </div>
            <Button type="submit" className="h-11 w-full" disabled={busy || locked}>
              {busy ? "Signing in…" : "Sign in"}
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
