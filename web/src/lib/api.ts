// api.ts — thin fetch wrapper around the Fluxa admin REST endpoints.
//
// The master key is held in sessionStorage rather than localStorage so
// it disappears with the tab: admins can bookmark the dashboard URL
// without worrying about a stale credential getting persisted across
// machine reboots. An explicit `setMasterKey` lets the login screen
// push the value once the user authenticates.

const MASTER_KEY_STORAGE = "fluxa-master-key";

export function getMasterKey(): string {
  return sessionStorage.getItem(MASTER_KEY_STORAGE) ?? "";
}

export function setMasterKey(key: string): void {
  if (key) {
    sessionStorage.setItem(MASTER_KEY_STORAGE, key);
  } else {
    sessionStorage.removeItem(MASTER_KEY_STORAGE);
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
      Authorization: `Bearer ${getMasterKey()}`,
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

// -- provider types + endpoints ----------------------------------------

export interface Provider {
  name: string;
  kind: string;
  api_key?: string;
  base_url?: string;
  api_version?: string;
  region?: string;
  models?: string[];
  enabled?: boolean;
  created_at?: string;
  updated_at?: string;
}

export const Providers = {
  list: () =>
    request<{ data: Provider[] }>("GET", "/admin/providers").then((r) => r.data ?? []),
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
