import { queryOptions } from "@tanstack/react-query";
import { z } from "zod";

import { ApiError, apiFetch, apiSend, setCsrfToken } from "@/lib/api";

export const sessionSchema = z.object({
  user: z.object({ id: z.string(), email: z.string(), role: z.enum(["seller", "reviewer"]) }),
  csrf_token: z.string().min(1),
  expires_at: z.string(),
});

export type Session = z.infer<typeof sessionSchema>;

export const sessionKeys = { current: ["session", "current"] as const };

/** fetchSession reads the current session; no session is null, not an error. */
export async function fetchSession(signal?: AbortSignal): Promise<Session | null> {
  try {
    const s = await apiFetch("/v1/sessions/current", sessionSchema, signal ? { signal } : {});
    setCsrfToken(s.csrf_token);
    return s;
  } catch (err) {
    if (err instanceof ApiError && err.status === 401) return null;
    throw err;
  }
}

export function sessionQueryOptions() {
  return queryOptions({
    queryKey: sessionKeys.current,
    queryFn: ({ signal }) => fetchSession(signal),
    staleTime: Infinity,
  });
}

/** signIn creates a session (POST /v1/sessions); the cookie is set by the API. */
export async function signIn(email: string, password: string): Promise<Session> {
  const s = await apiFetch("/v1/sessions", sessionSchema, {
    method: "POST",
    body: JSON.stringify({ email, password }),
  });
  setCsrfToken(s.csrf_token);
  return s;
}

/** signOut ends the session (DELETE /v1/sessions/current). */
export async function signOut(): Promise<void> {
  await apiSend("/v1/sessions/current", z.undefined(), { method: "DELETE" });
  setCsrfToken("");
}
