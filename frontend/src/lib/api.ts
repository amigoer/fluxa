// Thin fetch wrapper. Session auth is a cookie (DESIGN.md 7.1), so every
// request carries credentials; there is no token to attach by hand.
export class ApiError extends Error {
  status: number
  key: string

  constructor(status: number, key: string, detail?: string) {
    super(detail || key)
    this.status = status
    this.key = key
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    ...init,
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
      ...init?.headers,
    },
  })

  if (!res.ok) {
    let key = "common.internal_error"
    let detail: string | undefined
    try {
      const body = await res.json()
      key = body.key ?? key
      detail = body.detail
    } catch {
      // non-JSON error body; fall through with the generic key
    }
    throw new ApiError(res.status, key, detail)
  }

  if (res.status === 204 || res.headers.get("Content-Length") === "0") {
    return undefined as T
  }
  return res.json() as Promise<T>
}

export const api = {
  get: <T>(path: string) => request<T>(path),
  post: <T>(path: string, body?: unknown) =>
    request<T>(path, { method: "POST", body: body ? JSON.stringify(body) : undefined }),
  put: <T>(path: string, body?: unknown) =>
    request<T>(path, { method: "PUT", body: body ? JSON.stringify(body) : undefined }),
  patch: <T>(path: string, body?: unknown) =>
    request<T>(path, { method: "PATCH", body: body ? JSON.stringify(body) : undefined }),
  delete: <T>(path: string) => request<T>(path, { method: "DELETE" }),
}
