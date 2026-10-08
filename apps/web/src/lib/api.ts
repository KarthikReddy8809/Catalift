import type { z } from "zod";

import { env } from "@/lib/env";

export type ApiErrorCode = "network" | "http" | "invalid_json" | "invalid_response";

/** ApiError is the only error apiFetch throws; consumers switch on code and status. */
export class ApiError extends Error {
  readonly code: ApiErrorCode;
  readonly status: number;
  readonly details: unknown;
  /** Seconds from a 429's Retry-After header, when it had one. */
  readonly retryAfterSeconds: number | undefined;

  constructor(
    code: ApiErrorCode,
    status: number,
    message: string,
    details?: unknown,
    retryAfterSeconds?: number,
  ) {
    super(message);
    this.name = "ApiError";
    this.code = code;
    this.status = status;
    this.details = details;
    this.retryAfterSeconds = retryAfterSeconds;
  }
}

/**
 * apiFetch calls the API, validates the JSON body with the schema and returns
 * the typed value. Pass the query's signal so navigation cancels the request.
 */
export async function apiFetch<T>(
  path: string,
  schema: z.ZodType<T>,
  init: RequestInit = {},
): Promise<T> {
  const method = init.method ?? "GET";
  const headers = new Headers(init.headers);
  headers.set("Accept", "application/json");
  if (init.body !== undefined && !(init.body instanceof FormData) && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }

  let response: Response;
  try {
    response = await fetch(`${env.VITE_API_URL}${path}`, { ...init, headers });
  } catch (cause) {
    throw new ApiError("network", 0, `${method} ${path}: network error`, cause);
  }

  if (!response.ok) {
    const body = await response.text().catch(() => "");
    const retryAfter = Number(response.headers.get("Retry-After") ?? "");
    throw new ApiError(
      "http",
      response.status,
      `${method} ${path}: ${String(response.status)} ${response.statusText}`.trim(),
      body,
      Number.isFinite(retryAfter) && retryAfter > 0 ? retryAfter : undefined,
    );
  }

  // 204 and 205 have no body: the schema decides whether none is acceptable
  // (z.undefined() for a DELETE), so a no-content call still checks its status.
  if (response.status === 204 || response.status === 205) {
    const parsed = schema.safeParse(undefined);
    if (!parsed.success) {
      throw new ApiError(
        "invalid_response",
        response.status,
        `${method} ${path}: expected a body, got ${String(response.status)}`,
        parsed.error.issues,
      );
    }
    return parsed.data;
  }

  let json: unknown;
  try {
    json = await response.json();
  } catch (cause) {
    throw new ApiError(
      "invalid_json",
      response.status,
      `${method} ${path}: body is not JSON`,
      cause,
    );
  }

  const parsed = schema.safeParse(json);
  if (!parsed.success) {
    throw new ApiError(
      "invalid_response",
      response.status,
      `${method} ${path}: response failed validation`,
      parsed.error.issues,
    );
  }
  return parsed.data;
}

/** The API's error envelope (api/openapi.yaml Error). */
export interface ApiErrorBody {
  code: string;
  message: string;
  requestId: string;
  details: { field?: string; reason?: string }[];
}

/** errorBody reads the envelope from an ApiError's body, or null if there is none. */
export function errorBody(err: unknown): ApiErrorBody | null {
  if (!(err instanceof ApiError) || typeof err.details !== "string") return null;
  try {
    const parsed: unknown = JSON.parse(err.details);
    if (typeof parsed !== "object" || parsed === null || !("error" in parsed)) return null;
    const e = (parsed as { error: Record<string, unknown> }).error;
    return {
      code: typeof e.code === "string" ? e.code : "",
      message: typeof e.message === "string" ? e.message : "",
      requestId: typeof e.request_id === "string" ? e.request_id : "",
      details: Array.isArray(e.details) ? (e.details as ApiErrorBody["details"]) : [],
    };
  } catch {
    return null;
  }
}

let csrfToken = "";

/** setCsrfToken keeps the session's CSRF token for state-changing calls (ADR-0006). */
export function setCsrfToken(token: string): void {
  csrfToken = token;
}

interface SendOptions {
  method: "POST" | "PUT" | "PATCH" | "DELETE";
  /** A JSON body, or FormData for a file upload. */
  body?: unknown;
  /** Creates send an Idempotency-Key so a retried click is not done twice. */
  idempotent?: boolean;
}

/** apiSend is apiFetch for writes: it adds the CSRF token and, for creates, an Idempotency-Key. */
export function apiSend<T>(path: string, schema: z.ZodType<T>, opts: SendOptions): Promise<T> {
  const headers = new Headers({ "X-CSRF-Token": csrfToken });
  if (opts.idempotent) headers.set("Idempotency-Key", crypto.randomUUID());
  const body =
    opts.body === undefined
      ? undefined
      : opts.body instanceof FormData
        ? opts.body
        : JSON.stringify(opts.body);
  return apiFetch(path, schema, { method: opts.method, headers, ...(body ? { body } : {}) });
}
