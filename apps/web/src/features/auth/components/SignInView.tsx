import { PenLine, ScanSearch, ShieldCheck } from "lucide-react";

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { BrandMark } from "@/features/shell/components/AppFrame";

const PITCH = [
  { icon: ScanSearch, text: "Colour, pattern, sleeve and fit read from each photo." },
  { icon: PenLine, text: "A listing per channel, written in the brand's voice." },
  { icon: ShieldCheck, text: "Every listing checked against the channel's rules." },
];

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
    <div className="grid min-h-svh lg:grid-cols-2">
      <section
        aria-labelledby="pitch"
        className="relative hidden flex-col justify-between overflow-hidden bg-primary p-10 text-primary-foreground lg:flex"
      >
        <div
          aria-hidden
          className="pointer-events-none absolute -top-24 -right-24 size-96 rounded-full bg-brand-bright/30 blur-3xl"
        />
        <div
          aria-hidden
          className="pointer-events-none absolute -bottom-32 -left-16 size-96 rounded-full bg-brand-soft/20 blur-3xl"
        />
        <BrandMark className="relative text-lg [&>span:first-child]:bg-primary-foreground [&>span:first-child]:text-primary" />
        <div className="relative space-y-6">
          <h1 id="pitch" className="max-w-md text-4xl font-semibold tracking-tight text-balance">
            Product photos in, channel-ready listings out.
          </h1>
          <ul className="max-w-md space-y-3 text-primary-foreground/85">
            {PITCH.map(({ icon: Icon, text }) => (
              <li key={text} className="flex gap-3">
                <Icon className="mt-0.5 size-5 shrink-0" aria-hidden />
                <span>{text}</span>
              </li>
            ))}
          </ul>
        </div>
        <p className="relative text-sm text-primary-foreground/70">
          Every listing is approved by a reviewer before it is exported.
        </p>
      </section>

      <div className="flex items-center justify-center bg-gradient-to-b from-brand-soft/60 to-background p-6">
        <div className="w-full max-w-sm space-y-6">
          <BrandMark className="justify-center text-lg lg:hidden" />
          <Card className="shadow-lg">
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
                    Sign-in from this network is paused for {retryAfterSeconds ?? 0} seconds. Wait,
                    then try again.
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
      </div>
    </div>
  );
}
