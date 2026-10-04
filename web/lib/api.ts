// Minimal client for the identity service (contracts/openapi/tryst.v1.yaml). Requests go to
// same-origin /v1/* which Next.js proxies to identity-svc.

const TOKEN_KEY = "tryst.session";

export function getToken(): string | null {
  try {
    return sessionStorage.getItem(TOKEN_KEY);
  } catch {
    return null;
  }
}

export function setToken(t: string | null): void {
  try {
    if (t) sessionStorage.setItem(TOKEN_KEY, t);
    else sessionStorage.removeItem(TOKEN_KEY);
  } catch {
    /* storage unavailable: session lives only for this page */
  }
}

export type ApiResult<T = Record<string, unknown>> = { status: number; data: T; headers: Headers };

export async function api<T = Record<string, unknown>>(
  path: string,
  opts: { method?: string; body?: unknown; token?: string | null; headers?: Record<string, string> } = {},
): Promise<ApiResult<T>> {
  const headers: Record<string, string> = { "Content-Type": "application/json", ...(opts.headers ?? {}) };
  if (opts.token) headers.Authorization = `Bearer ${opts.token}`;
  const res = await fetch(path, {
    method: opts.method ?? "GET",
    headers,
    body: opts.body === undefined ? undefined : JSON.stringify(opts.body),
    cache: "no-store",
  });
  const text = await res.text();
  let data = {} as T;
  if (text) {
    try {
      data = JSON.parse(text) as T;
    } catch {
      data = {} as T;
    }
  }
  return { status: res.status, data, headers: res.headers };
}

/** Problem+json title for display; falls back to a generic message. */
export function problemTitle(data: unknown, fallback = "Something went wrong. Please try again."): string {
  if (data && typeof data === "object" && "title" in data && typeof (data as { title: unknown }).title === "string") {
    return (data as { title: string }).title;
  }
  return fallback;
}
