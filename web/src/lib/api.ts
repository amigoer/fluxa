// api.ts — thin fetch wrapper around the Fluxa admin REST endpoints.
//
// Authentication moved from a static master key to username + password
// in v2.2: callers POST /admin/auth/login, get back an opaque session
// token, and present it as Authorization: Bearer <token> on every
// subsequent call. We hold the token in localStorage so the dashboard
// survives a page reload (the server-side TTL is one week, after which
// the user signs in again anyway).

const TOKEN_STORAGE = "fluxa-session-token";

export function getSessionToken(): string {
  return localStorage.getItem(TOKEN_STORAGE) ?? "";
}

export function setSessionToken(token: string): void {
  if (token) {
    localStorage.setItem(TOKEN_STORAGE, token);
  } else {
    localStorage.removeItem(TOKEN_STORAGE);
  }
}

async function request<T>(
  method: string,
  path: string,
  body?: unknown,
): Promise<T> {
  const res = await fetch(path, {
    method,
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${getSessionToken()}`,
    },
    body: body ? JSON.stringify(body) : undefined,
  });
  if (res.status === 204) {
    return undefined as T;
  }
  const text = await res.text();
  const data = text ? JSON.parse(text) : null;
  if (!res.ok) {
    const message = data?.error?.message ?? res.statusText;
    throw new Error(message);
  }
  return data as T;
}

// -- auth types + endpoints --------------------------------------------

export interface AdminUser {
  id: number;
  username: string;
  created_at?: string;
}

export interface LoginResponse {
  token: string;
  expires_at: string;
  user: AdminUser;
}

export const Auth = {
  login: async (username: string, password: string): Promise<LoginResponse> => {
    const res = await fetch("/admin/auth/login", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ username, password }),
    });
    const text = await res.text();
    const data = text ? JSON.parse(text) : null;
    if (!res.ok) {
      throw new Error(data?.error?.message ?? res.statusText);
    }
    setSessionToken(data.token);
    return data as LoginResponse;
  },
  logout: async (): Promise<void> => {
    try {
      await request<void>("POST", "/admin/auth/logout");
    } catch {
      // Logout is best-effort: even if the server rejects the call we
      // still clear local state below so the UI returns to the login
      // screen.
    }
    setSessionToken("");
  },
  me: () => request<AdminUser>("GET", "/admin/auth/me"),
  changePassword: (oldPassword: string, newPassword: string) =>
    request<void>("POST", "/admin/auth/password", {
      old_password: oldPassword,
      new_password: newPassword,
    }),
};

// -- provider types + endpoints ----------------------------------------

// Provider mirrors the backend providerDTO 1:1 so the dashboard can edit
// every field the gateway supports — no more "edit-via-YAML-only" gaps
// for AWS Bedrock credentials, Azure deployments, custom headers, etc.
export interface Provider {
  name: string;
  kind: string;
  api_key?: string;
  base_url?: string;
  // Azure / OpenAI-compatible fields.
  api_version?: string;
  // AWS Bedrock SigV4 credentials. Region is also reused by other
  // region-aware backends.
  region?: string;
  access_key?: string;
  secret_key?: string;
  session_token?: string;
  // Azure: model-name → deployment-name map. Free-form key/value pairs.
  deployments?: Record<string, string>;
  // Optional advertised model list (used by the router's "find any
  // provider that serves this model" fallback when no explicit route
  // entry exists).
  models?: string[];
  // Extra HTTP headers injected on every upstream request.
  headers?: Record<string, string>;
  // Per-request timeout in seconds. 0 = use the gateway default.
  timeout_sec?: number;
  enabled?: boolean;
  created_at?: string;
  updated_at?: string;
}

export const Providers = {
  list: () =>
    request<{ data: Provider[] }>("GET", "/admin/providers").then((r) => r.data ?? []),
  // POST is upsert: the backend treats the body's `name` as the primary
  // key, so the same call serves both create and edit flows.
  upsert: (p: Provider) => request<Provider>("POST", "/admin/providers", p),
  delete: (name: string) => request<void>("DELETE", `/admin/providers/${name}`),
};

// -- route types + endpoints -------------------------------------------

export interface Route {
  model: string;
  provider: string;
  fallback?: string[];
  created_at?: string;
  updated_at?: string;
}

export const Routes = {
  list: () =>
    request<{ data: Route[] }>("GET", "/admin/routes").then((r) => r.data ?? []),
  upsert: (r: Route) => request<Route>("POST", "/admin/routes", r),
  delete: (model: string) => request<void>("DELETE", `/admin/routes/${model}`),
};

// -- virtual model types + endpoints -----------------------------------
//
// Virtual models are user-facing aliases that fan out to one or more
// concrete targets with weighted random picking. They live behind
// /admin/virtual-models and the data plane resolves them in front of
// the legacy provider chain — see internal/router/model_resolver.go.

export interface VirtualModelRoute {
  id?: string;
  weight: number;
  target_type: "real" | "virtual";
  target_model: string;
  provider?: string;
  enabled?: boolean;
  position?: number;
}

export interface VirtualModel {
  id?: string;
  name: string;
  description?: string;
  enabled?: boolean;
  routes: VirtualModelRoute[];
  created_at?: string;
  updated_at?: string;
}

export const VirtualModels = {
  list: () =>
    request<{ data: VirtualModel[] }>("GET", "/admin/virtual-models").then(
      (r) => r.data ?? [],
    ),
  // POST is upsert by name (full replace of the routes list).
  upsert: (vm: VirtualModel) =>
    request<VirtualModel>("POST", "/admin/virtual-models", vm),
  delete: (name: string) =>
    request<void>("DELETE", `/admin/virtual-models/${name}`),
};

// -- regex route types + endpoints -------------------------------------

export interface RegexRoute {
  id?: string;
  pattern: string;
  priority: number;
  target_type: "real" | "virtual";
  target_model: string;
  provider?: string;
  description?: string;
  enabled?: boolean;
  created_at?: string;
  updated_at?: string;
}

export const RegexRoutes = {
  list: () =>
    request<{ data: RegexRoute[] }>("GET", "/admin/regex-routes").then(
      (r) => r.data ?? [],
    ),
  create: (r: RegexRoute) =>
    request<RegexRoute>("POST", "/admin/regex-routes", r),
  update: (id: string, r: RegexRoute) =>
    request<RegexRoute>("PUT", `/admin/regex-routes/${id}`, r),
  setPriority: (id: string, priority: number) =>
    request<void>("PATCH", `/admin/regex-routes/${id}/priority`, { priority }),
  delete: (id: string) => request<void>("DELETE", `/admin/regex-routes/${id}`),
};

// -- resolve tester ----------------------------------------------------

export interface ResolveTraceStep {
  depth: number;
  type: "regex_match" | "virtual_model" | "passthrough";
  pattern?: string;
  name?: string;
  target_type?: string;
  target?: string;
  provider?: string;
  weight_picked?: number;
}

export interface ResolveResult {
  input: string;
  passthrough: boolean;
  target?: { provider: string; model: string };
  trace: ResolveTraceStep[];
  error?: string;
}

export const Resolver = {
  test: (model: string) =>
    request<ResolveResult>(
      "GET",
      `/admin/resolve-model?model=${encodeURIComponent(model)}`,
    ),
};

// -- virtual key types + endpoints -------------------------------------

export interface VirtualKey {
  id?: string;
  name: string;
  description?: string;
  models?: string[];
  ip_allowlist?: string[];
  budget_tokens_daily?: number;
  budget_tokens_monthly?: number;
  budget_usd_daily?: number;
  budget_usd_monthly?: number;
  rpm_limit?: number;
  enabled?: boolean;
  expires_at?: string;
  created_at?: string;
  updated_at?: string;
}

export const Keys = {
  list: () =>
    request<{ data: VirtualKey[] }>("GET", "/admin/keys").then((r) => r.data ?? []),
  create: (k: VirtualKey) => request<VirtualKey>("POST", "/admin/keys", k),
  update: (id: string, patch: Partial<VirtualKey>) =>
    request<VirtualKey>("PUT", `/admin/keys/${id}`, patch),
  delete: (id: string) => request<void>("DELETE", `/admin/keys/${id}`),
};

// -- usage types + endpoints -------------------------------------------

export interface UsageRecord {
  ID: number;
  VirtualKeyID: string;
  Ts: string;
  Model: string;
  Provider: string;
  PromptTokens: number;
  CompletionTokens: number;
  TotalTokens: number;
  CostUSD: number;
  LatencyMs: number;
  Status: number;
}

export interface UsageTotals {
  Tokens: number;
  PromptTokens: number;
  CompletionTokens: number;
  CostUSD: number;
  Requests: number;
}

export interface UsageSummary {
  key_id: string;
  daily: UsageTotals;
  monthly: UsageTotals;
}

// -- config import / export --------------------------------------------
//
// The gateway boots from env vars only, but operators still want a
// human-readable snapshot format for backup and bulk edits. Export
// fetches the live store as a YAML bundle and returns it as a raw
// string so the caller can drop it into a download link or a
// textarea. Import accepts the same shape back: it is a plain text
// POST rather than JSON because the body is YAML, not a DTO.

export const Config = {
  export: async (): Promise<string> => {
    const res = await fetch("/admin/config/export", {
      headers: { Authorization: `Bearer ${getSessionToken()}` },
    });
    if (!res.ok) {
      const text = await res.text();
      throw new Error(text || res.statusText);
    }
    return res.text();
  },
  import: async (yaml: string): Promise<{ providers: number; routes: number }> => {
    const res = await fetch("/admin/config/import", {
      method: "POST",
      headers: {
        "Content-Type": "application/yaml",
        Authorization: `Bearer ${getSessionToken()}`,
      },
      body: yaml,
    });
    const text = await res.text();
    const data = text ? JSON.parse(text) : {};
    if (!res.ok) {
      throw new Error(data?.error?.message ?? res.statusText);
    }
    return { providers: data.providers ?? 0, routes: data.routes ?? 0 };
  },
};

export const Usage = {
  list: (keyID?: string, limit = 100) => {
    const q = new URLSearchParams();
    if (keyID) q.set("key_id", keyID);
    q.set("limit", String(limit));
    return request<{ data: UsageRecord[] }>("GET", `/admin/usage?${q}`).then(
      (r) => r.data ?? [],
    );
  },
  summary: (keyID?: string) => {
    const q = new URLSearchParams();
    if (keyID) q.set("key_id", keyID);
    const path = `/admin/usage/summary${q.toString() ? "?" + q : ""}`;
    return request<UsageSummary>("GET", path);
  },
};
