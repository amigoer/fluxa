// Typed client for the gateway's /admin REST surface.
//
// Auth is a session token minted by POST /admin/auth/login and sent as a
// bearer header on every later call. The token lives in localStorage: the
// console is a static bundle served by the same Go binary that owns the
// API, so there is no server-side session to keep it in.

const TOKEN_KEY = "fluxa-token";

export function getToken(): string | null {
  try {
    return localStorage.getItem(TOKEN_KEY);
  } catch {
    return null;
  }
}

export function setToken(token: string | null) {
  try {
    if (token) localStorage.setItem(TOKEN_KEY, token);
    else localStorage.removeItem(TOKEN_KEY);
  } catch {
    // Storage unavailable: the session simply will not survive a reload.
  }
}

/** Thrown for any non-2xx response so callers can branch on status. */
export class ApiError extends Error {
  // A plain field rather than a constructor parameter property: the
  // tsconfig runs with erasableSyntaxOnly, which bans the shorthand.
  status: number;

  constructor(status: number, message: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
  }
}

/** Fired when the gateway rejects our token, so the app can bounce to /login. */
export const UNAUTHORIZED_EVENT = "fluxa:unauthorized";

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const token = getToken();
  const response = await fetch(path, {
    method,
    headers: {
      ...(body === undefined ? {} : { "Content-Type": "application/json" }),
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    body: body === undefined ? undefined : JSON.stringify(body),
  });

  if (response.status === 401) {
    setToken(null);
    window.dispatchEvent(new CustomEvent(UNAUTHORIZED_EVENT));
    throw new ApiError(401, "Session expired — please sign in again");
  }

  if (!response.ok) {
    // The gateway answers with {"error":{"message","type"}}; fall back to
    // the status line for anything that is not shaped like that.
    let message = `${response.status} ${response.statusText}`;
    try {
      const payload = await response.json();
      if (payload?.error?.message) message = payload.error.message;
    } catch {
      /* non-JSON body */
    }
    throw new ApiError(response.status, message);
  }

  if (response.status === 204) return undefined as T;
  return (await response.json()) as T;
}

const get = <T,>(path: string) => request<T>("GET", path);
const post = <T,>(path: string, body?: unknown) => request<T>("POST", path, body);
const put = <T,>(path: string, body?: unknown) => request<T>("PUT", path, body);
const patch = <T,>(path: string, body?: unknown) => request<T>("PATCH", path, body);
const del = <T,>(path: string) => request<T>("DELETE", path);

// -- types ----------------------------------------------------------------
// These mirror the DTOs in internal/api. Optional fields use `omitempty`
// on the Go side, so anything not required is optional here too.

export type User = {
  id: number;
  username: string;
  nickname: string;
  email: string;
  avatar_url: string;
  created_at?: string;
};

export type LoginResponse = { token: string; expires_at: string; user: User };

export type Provider = {
  name: string;
  kind: string;
  api_key?: string;
  base_url?: string;
  api_version?: string;
  region?: string;
  access_key?: string;
  secret_key?: string;
  session_token?: string;
  deployments?: Record<string, string>;
  models?: string[];
  headers?: Record<string, string>;
  timeout_sec?: number;
  enabled?: boolean;
  created_at?: string;
  updated_at?: string;
};

export type Route = {
  model: string;
  provider: string;
  fallback?: string[];
  created_at?: string;
  updated_at?: string;
};

export type VirtualModelRoute = {
  id?: string;
  weight: number;
  target_type: "real" | "virtual";
  target_model: string;
  provider?: string;
  enabled?: boolean;
  position?: number;
};

export type VirtualModel = {
  id?: string;
  name: string;
  description?: string;
  enabled?: boolean;
  routes: VirtualModelRoute[];
  created_at?: string;
  updated_at?: string;
};

export type RegexModel = {
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
};

export type VirtualKey = {
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
  expires_at?: string | null;
  created_at?: string;
  updated_at?: string;
};

/** store.UsageTotals has no json tags, so Go marshals the field names as-is. */
export type UsageTotals = {
  Tokens: number;
  PromptTokens: number;
  CompletionTokens: number;
  CostUSD: number;
  Requests: number;
};

export type UsageSummary = { key_id: string; daily: UsageTotals; monthly: UsageTotals };

/** One day of the overview trend line; days with no traffic are zero-filled. */
export type AnalyticsBucket = {
  date: string;
  requests: number;
  tokens: number;
  cost_usd: number;
  errors: number;
};

export type AnalyticsBreakdown = {
  name: string;
  requests: number;
  tokens: number;
  cost_usd: number;
};

export type AnalyticsTotals = {
  requests: number;
  tokens: number;
  cost_usd: number;
  errors: number;
};

/** Everything the overview renders, in one round trip. */
export type AnalyticsOverview = {
  days: number;
  totals: AnalyticsTotals;
  /** Equal-length window immediately before `totals`, for trend deltas. */
  previous: AnalyticsTotals;
  series: AnalyticsBucket[];
  by_provider: AnalyticsBreakdown[];
  by_model: AnalyticsBreakdown[];
};

export type RequestLog = {
  id: string;
  virtual_key_id: string;
  started_at: string;
  first_byte_at?: string;
  completed_at: string;
  endpoint: string;
  method: string;
  model_requested: string;
  model_resolved: string;
  provider: string;
  is_stream: boolean;
  cache_hit: boolean;
  status_code: number;
  error: string;
  prompt_tokens: number;
  completion_tokens: number;
  total_tokens: number;
  cost_usd: number;
  latency_ms: number;
  ttft_ms: number;
  client_ip: string;
  user_agent: string;
};

export type RequestLogDetail = RequestLog & { request_body: string; response_body: string };

export type DLPRule = {
  id?: string;
  name: string;
  pattern: string;
  pattern_type: "keyword" | "regex";
  scope: "request" | "response" | "both";
  action: "block" | "mask" | "log";
  priority: number;
  model_pattern?: string;
  description?: string;
  enabled?: boolean;
  created_at?: string;
  updated_at?: string;
};

export type DLPViolation = {
  id: number;
  rule_id: string;
  rule_name: string;
  key_id: string;
  model: string;
  direction: string;
  matched_text: string;
  action_taken: string;
  created_at: string;
};

export type RequestLogFilter = {
  key_id?: string;
  model?: string;
  provider?: string;
  status_min?: number;
  status_max?: number;
  stream?: boolean;
  search?: string;
  limit?: number;
  offset?: number;
};

/** Drops empty values so the query string only carries active filters. */
function query(params: Record<string, string | number | boolean | undefined>) {
  const search = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value === undefined || value === "" ) continue;
    search.set(key, String(value));
  }
  const encoded = search.toString();
  return encoded ? `?${encoded}` : "";
}

// -- endpoints ------------------------------------------------------------

export const api = {
  login: (username: string, password: string) =>
    post<LoginResponse>("/admin/auth/login", { username, password }),
  logout: () => post<unknown>("/admin/auth/logout"),
  me: () => get<User>("/admin/auth/me"),
  updateProfile: (body: { nickname: string; email: string; avatar_url: string }) =>
    put<User>("/admin/auth/profile", body),
  changePassword: (old_password: string, new_password: string) =>
    post<unknown>("/admin/auth/password", { old_password, new_password }),

  listProviders: () => get<{ data: Provider[] }>("/admin/providers").then((r) => r.data ?? []),
  upsertProvider: (p: Provider) => put<Provider>(`/admin/providers/${encodeURIComponent(p.name)}`, p),
  deleteProvider: (name: string) => del<unknown>(`/admin/providers/${encodeURIComponent(name)}`),

  listRoutes: () => get<{ data: Route[] }>("/admin/routes").then((r) => r.data ?? []),
  upsertRoute: (r: Route) => put<Route>(`/admin/routes/${encodeURIComponent(r.model)}`, r),
  deleteRoute: (model: string) => del<unknown>(`/admin/routes/${encodeURIComponent(model)}`),

  listVirtualModels: () =>
    get<{ data: VirtualModel[] }>("/admin/virtual-models").then((r) => r.data ?? []),
  upsertVirtualModel: (vm: VirtualModel) =>
    put<VirtualModel>(`/admin/virtual-models/${encodeURIComponent(vm.name)}`, vm),
  deleteVirtualModel: (name: string) =>
    del<unknown>(`/admin/virtual-models/${encodeURIComponent(name)}`),

  listRegexModels: () =>
    get<{ data: RegexModel[] }>("/admin/regex-models").then((r) => r.data ?? []),
  createRegexModel: (m: RegexModel) => post<RegexModel>("/admin/regex-models", m),
  updateRegexModel: (m: RegexModel) => put<RegexModel>(`/admin/regex-models/${m.id}`, m),
  deleteRegexModel: (id: string) => del<unknown>(`/admin/regex-models/${id}`),

  listKeys: () => get<{ data: VirtualKey[] }>("/admin/keys").then((r) => r.data ?? []),
  createKey: (k: VirtualKey) => post<VirtualKey>("/admin/keys", k),
  updateKey: (k: VirtualKey) => put<VirtualKey>(`/admin/keys/${k.id}`, k),
  deleteKey: (id: string) => del<unknown>(`/admin/keys/${id}`),

  usageSummary: (keyID?: string) =>
    get<UsageSummary>(`/admin/usage/summary${query({ key_id: keyID })}`),

  analyticsOverview: (days: number) =>
    get<AnalyticsOverview>(`/admin/analytics/overview${query({ days })}`),

  listLogs: (filter: RequestLogFilter) =>
    get<{ data: RequestLog[]; total: number; limit: number; offset: number }>(
      `/admin/logs${query(filter)}`,
    ),
  getLog: (id: string) => get<RequestLogDetail>(`/admin/logs/${id}`),

  listDLPRules: () => get<{ data: DLPRule[] }>("/admin/dlp-rules").then((r) => r.data ?? []),
  createDLPRule: (rule: DLPRule) => post<DLPRule>("/admin/dlp-rules", rule),
  updateDLPRule: (rule: DLPRule) => put<DLPRule>(`/admin/dlp-rules/${rule.id}`, rule),
  updateDLPRulePriority: (id: string, priority: number) =>
    patch<DLPRule>(`/admin/dlp-rules/${id}/priority`, { priority }),
  deleteDLPRule: (id: string) => del<unknown>(`/admin/dlp-rules/${id}`),
  listDLPViolations: (limit = 50, offset = 0, ruleID?: string) =>
    get<{ data: DLPViolation[]; total: number }>(
      `/admin/dlp-violations${query({ limit, offset, rule_id: ruleID })}`,
    ),

  resolveModel: (model: string) =>
    get<Record<string, unknown>>(`/admin/resolve-model${query({ model })}`),

  reload: () => post<unknown>("/admin/reload"),
};
