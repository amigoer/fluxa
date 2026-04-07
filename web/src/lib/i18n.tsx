// i18n.tsx — tiny in-house translation layer for the dashboard.
//
// We deliberately do not pull in i18next or react-intl: the full string
// surface fits in one file, the dashboard ships as a single embedded
// SPA, and adding 200 KB of dependency for ~120 strings is not the
// trade-off we want. Instead this module exposes:
//
//   - I18nProvider: a context that holds the active locale ("en" | "zh")
//     and persists the user's choice in localStorage so it survives
//     page reloads.
//   - useT(): returns a `t(key)` function plus the active locale and a
//     setLocale setter for the language toggle UI.
//   - All keys live in the `dict` object below; the type system enforces
//     that every locale supplies every key, so a missing translation is
//     a build-time error rather than a runtime "??missing??".

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";

export type Locale = "en" | "zh";

const STORAGE_KEY = "fluxa-locale";

// dict is the single source of truth for every visible string. Add a
// new entry here, then reference it via t("yourKey") anywhere in the
// app. Both locales must define the key — TypeScript will reject any
// drift across the two records.
const dict = {
  en: {
    "app.title": "Fluxa",
    "app.subtitle": "AI gateway admin",

    "nav.dashboard": "Dashboard",
    "nav.providers": "Providers",
    "nav.routes": "Routes",
    "nav.keys": "Virtual keys",
    "nav.usage": "Usage",
    "nav.settings": "Settings",
    "nav.signOut": "Sign out",
    "nav.account": "Signed in as",

    "lang.toggle": "中文",

    "login.title": "Fluxa admin",
    "login.username": "Username",
    "login.password": "Password",
    "login.submit": "Sign in",
    "login.checking": "Checking…",
    "login.placeholderUser": "admin",
    "login.placeholderPass": "Your password",
    "login.invalid": "Invalid username or password",
    "login.failed": "Login failed",
    "login.firstRunHint":
      "First run? Default credentials are admin / admin — change them in Settings after sign-in.",

    "dashboard.title": "Dashboard",
    "dashboard.subtitle": "Fleet-wide snapshot of routing state and key usage.",
    "dashboard.providers": "Providers",
    "dashboard.routes": "Routes",
    "dashboard.keys": "Virtual keys",
    "dashboard.usageToday": "Usage today",
    "dashboard.usageMonth": "Usage this month",
    "dashboard.requests": "Requests",
    "dashboard.promptTokens": "Prompt tokens",
    "dashboard.completionTokens": "Completion tokens",
    "dashboard.totalTokens": "Total tokens",
    "dashboard.costUSD": "Cost (USD)",

    "providers.title": "Providers",
    "providers.subtitle": "Upstream vendors Fluxa can route to.",
    "providers.new": "New provider",
    "providers.colName": "Name",
    "providers.colKind": "Kind",
    "providers.colBaseURL": "Base URL",
    "providers.colStatus": "Status",
    "providers.statusEnabled": "enabled",
    "providers.statusDisabled": "disabled",
    "providers.empty": "No providers yet.",
    "providers.deleteConfirm": "Delete provider {name}?",
    "providers.fieldName": "Name",
    "providers.fieldKind": "Kind",
    "providers.fieldAPIKey": "API key",
    "providers.fieldBaseURL": "Base URL (optional)",

    "routes.title": "Routes",
    "routes.subtitle": "Model-name → provider mapping with optional fallback chain.",
    "routes.new": "New route",
    "routes.colModel": "Model",
    "routes.colProvider": "Provider",
    "routes.colFallback": "Fallback",
    "routes.empty": "No routes yet.",
    "routes.deleteConfirm": "Delete route for {model}?",
    "routes.fieldFallback": "Fallback (comma-separated)",

    "keys.title": "Virtual keys",
    "keys.subtitle": "Per-application credentials with rate limits and budgets.",
    "keys.new": "New key",
    "keys.colName": "Name",
    "keys.colId": "ID",
    "keys.colModels": "Models",
    "keys.colRPM": "RPM",
    "keys.colStatus": "Status",
    "keys.empty": "No virtual keys yet.",
    "keys.deleteConfirm": "Delete key {id}? This also removes its usage history.",
    "keys.copyOnce": "Copy this key — it will not be shown again",
    "keys.copy": "Copy",
    "keys.dismiss": "Dismiss",
    "keys.fieldName": "Name",
    "keys.fieldDescription": "Description",
    "keys.fieldModels": "Models (comma-separated, * for any)",
    "keys.fieldIPs": "IP allowlist (CIDR or exact, comma-separated)",
    "keys.fieldRPM": "RPM limit",
    "keys.fieldDailyTokens": "Daily tokens budget",
    "keys.fieldMonthlyTokens": "Monthly tokens budget",
    "keys.fieldDailyUSD": "Daily USD budget",
    "keys.fieldMonthlyUSD": "Monthly USD budget",
    "keys.formTitle": "New virtual key",
    "keys.create": "Create",

    "usage.title": "Usage",
    "usage.subtitle": "Recent per-request accounting. Up to 200 rows per view.",
    "usage.filterLabel": "Filter by virtual key id",
    "usage.refresh": "Refresh",
    "usage.colTime": "Time",
    "usage.colKey": "Key",
    "usage.colModel": "Model",
    "usage.colProvider": "Provider",
    "usage.colPrompt": "Prompt",
    "usage.colCompletion": "Completion",
    "usage.colTotal": "Total",
    "usage.colUSD": "USD",
    "usage.colLatency": "Latency",
    "usage.empty": "No usage recorded yet.",

    "settings.title": "Settings",
    "settings.subtitle":
      "Snapshot the full provider + route state as YAML, or bulk-restore a saved bundle. Same format as legacy v1.x fluxa.yaml — imports round-trip cleanly.",
    "settings.bundleTitle": "Config bundle",
    "settings.loadFromGateway": "Load from gateway",
    "settings.download": "Download YAML",
    "settings.applyImport": "Apply import",
    "settings.loaded": "Loaded current config from gateway.",
    "settings.imported": "Imported {providers} provider(s) and {routes} route(s).",
    "settings.pasteFirst": "Paste a YAML bundle first.",
    "settings.envHint":
      "Environment variables of the form ${VAR} and ${VAR:-default} are expanded on import, so you can keep secrets out of the YAML and inject them at runtime.",

    "settings.accountTitle": "Account",
    "settings.accountSubtitle": "Rotate the password used for this dashboard.",
    "settings.fieldOldPass": "Current password",
    "settings.fieldNewPass": "New password",
    "settings.fieldConfirmPass": "Confirm new password",
    "settings.changePass": "Change password",
    "settings.passChanged": "Password changed. Please sign in again.",
    "settings.passMismatch": "New password and confirmation do not match.",
    "settings.passTooShort": "Password must be at least 4 characters.",

    "common.save": "Save",
    "common.cancel": "Cancel",
    "common.loadFailed": "load failed",
    "common.saveFailed": "save failed",
    "common.deleteFailed": "delete failed",
    "common.toggleFailed": "toggle failed",
  },
  zh: {
    "app.title": "Fluxa",
    "app.subtitle": "AI 网关管理后台",

    "nav.dashboard": "概览",
    "nav.providers": "上游供应商",
    "nav.routes": "路由",
    "nav.keys": "虚拟密钥",
    "nav.usage": "用量",
    "nav.settings": "设置",
    "nav.signOut": "退出登录",
    "nav.account": "当前用户：",

    "lang.toggle": "English",

    "login.title": "Fluxa 管理后台",
    "login.username": "账号",
    "login.password": "密码",
    "login.submit": "登录",
    "login.checking": "登录中…",
    "login.placeholderUser": "admin",
    "login.placeholderPass": "请输入密码",
    "login.invalid": "账号或密码错误",
    "login.failed": "登录失败",
    "login.firstRunHint":
      "首次启动？默认账号 admin / admin，登录后请立即在「设置」中修改密码。",

    "dashboard.title": "概览",
    "dashboard.subtitle": "整个网关的路由状态与密钥用量快照。",
    "dashboard.providers": "供应商",
    "dashboard.routes": "路由",
    "dashboard.keys": "虚拟密钥",
    "dashboard.usageToday": "今日用量",
    "dashboard.usageMonth": "本月用量",
    "dashboard.requests": "请求数",
    "dashboard.promptTokens": "输入 token",
    "dashboard.completionTokens": "输出 token",
    "dashboard.totalTokens": "总 token",
    "dashboard.costUSD": "费用（美元）",

    "providers.title": "上游供应商",
    "providers.subtitle": "Fluxa 可以转发请求的目标厂商。",
    "providers.new": "新建供应商",
    "providers.colName": "名称",
    "providers.colKind": "类型",
    "providers.colBaseURL": "Base URL",
    "providers.colStatus": "状态",
    "providers.statusEnabled": "已启用",
    "providers.statusDisabled": "已停用",
    "providers.empty": "暂无供应商。",
    "providers.deleteConfirm": "确认删除供应商 {name} 吗？",
    "providers.fieldName": "名称",
    "providers.fieldKind": "类型",
    "providers.fieldAPIKey": "API key",
    "providers.fieldBaseURL": "Base URL（可选）",

    "routes.title": "路由",
    "routes.subtitle": "模型名 → 供应商 的映射，可配置回退链。",
    "routes.new": "新建路由",
    "routes.colModel": "模型",
    "routes.colProvider": "供应商",
    "routes.colFallback": "回退",
    "routes.empty": "暂无路由。",
    "routes.deleteConfirm": "确认删除模型 {model} 的路由吗？",
    "routes.fieldFallback": "回退（用英文逗号分隔）",

    "keys.title": "虚拟密钥",
    "keys.subtitle": "为每个应用单独发放的凭证，可配置速率限制与预算。",
    "keys.new": "新建密钥",
    "keys.colName": "名称",
    "keys.colId": "ID",
    "keys.colModels": "可用模型",
    "keys.colRPM": "RPM",
    "keys.colStatus": "状态",
    "keys.empty": "暂无虚拟密钥。",
    "keys.deleteConfirm": "确认删除密钥 {id}？同时会清除其用量历史。",
    "keys.copyOnce": "请立即复制此密钥，关闭后将无法再次查看",
    "keys.copy": "复制",
    "keys.dismiss": "关闭",
    "keys.fieldName": "名称",
    "keys.fieldDescription": "描述",
    "keys.fieldModels": "可用模型（用英文逗号分隔，* 表示全部）",
    "keys.fieldIPs": "IP 白名单（CIDR 或具体 IP，用英文逗号分隔）",
    "keys.fieldRPM": "RPM 限制",
    "keys.fieldDailyTokens": "每日 token 预算",
    "keys.fieldMonthlyTokens": "每月 token 预算",
    "keys.fieldDailyUSD": "每日费用预算（美元）",
    "keys.fieldMonthlyUSD": "每月费用预算（美元）",
    "keys.formTitle": "新建虚拟密钥",
    "keys.create": "创建",

    "usage.title": "用量",
    "usage.subtitle": "最近的每请求计量记录，单视图最多 200 条。",
    "usage.filterLabel": "按虚拟密钥 ID 过滤",
    "usage.refresh": "刷新",
    "usage.colTime": "时间",
    "usage.colKey": "密钥",
    "usage.colModel": "模型",
    "usage.colProvider": "供应商",
    "usage.colPrompt": "输入",
    "usage.colCompletion": "输出",
    "usage.colTotal": "合计",
    "usage.colUSD": "美元",
    "usage.colLatency": "耗时",
    "usage.empty": "暂无用量记录。",

    "settings.title": "设置",
    "settings.subtitle":
      "将完整的供应商 + 路由状态导出为 YAML，或粘贴一个备份批量恢复。格式与历史版本的 fluxa.yaml 兼容，导入导出可无损往返。",
    "settings.bundleTitle": "配置包",
    "settings.loadFromGateway": "从网关加载",
    "settings.download": "下载 YAML",
    "settings.applyImport": "应用导入",
    "settings.loaded": "已从网关加载当前配置。",
    "settings.imported": "已导入 {providers} 个供应商和 {routes} 条路由。",
    "settings.pasteFirst": "请先粘贴 YAML 配置内容。",
    "settings.envHint":
      "导入时会展开 ${VAR} 和 ${VAR:-default} 形式的环境变量，因此 YAML 里可以不写死密钥，运行时再注入。",

    "settings.accountTitle": "账号",
    "settings.accountSubtitle": "修改登录该后台的密码。",
    "settings.fieldOldPass": "当前密码",
    "settings.fieldNewPass": "新密码",
    "settings.fieldConfirmPass": "确认新密码",
    "settings.changePass": "修改密码",
    "settings.passChanged": "密码已更新，请重新登录。",
    "settings.passMismatch": "新密码两次输入不一致。",
    "settings.passTooShort": "密码长度至少 4 位。",

    "common.save": "保存",
    "common.cancel": "取消",
    "common.loadFailed": "加载失败",
    "common.saveFailed": "保存失败",
    "common.deleteFailed": "删除失败",
    "common.toggleFailed": "切换失败",
  },
} as const;

// TranslationKey is the union of every key declared in the EN dictionary.
// Because both records share the same keys (TypeScript verifies via the
// `satisfies` indirection below), this type covers both locales.
export type TranslationKey = keyof (typeof dict)["en"];

// Compile-time guarantee: ZH must mirror every key from EN.
const _zhCheck: Record<TranslationKey, string> = dict.zh;
void _zhCheck;

interface I18nValue {
  locale: Locale;
  setLocale: (l: Locale) => void;
  t: (key: TranslationKey, params?: Record<string, string | number>) => string;
}

const I18nContext = createContext<I18nValue | undefined>(undefined);

// I18nProvider wraps the application root and exposes the t() helper to
// any descendant. Initial locale is read from localStorage, then falls
// back to the browser's preferred language, then to English.
export function I18nProvider({ children }: { children: ReactNode }) {
  const [locale, setLocaleState] = useState<Locale>(() => {
    const stored = localStorage.getItem(STORAGE_KEY);
    if (stored === "en" || stored === "zh") return stored;
    if (typeof navigator !== "undefined" && navigator.language?.toLowerCase().startsWith("zh")) {
      return "zh";
    }
    return "en";
  });

  useEffect(() => {
    document.documentElement.lang = locale === "zh" ? "zh-CN" : "en";
  }, [locale]);

  const setLocale = useCallback((l: Locale) => {
    setLocaleState(l);
    localStorage.setItem(STORAGE_KEY, l);
  }, []);

  const t = useCallback(
    (key: TranslationKey, params?: Record<string, string | number>) => {
      const template = dict[locale][key] ?? key;
      if (!params) return template;
      // Tiny mustache: {name} → params.name, no escaping fanciness.
      return template.replace(/\{(\w+)\}/g, (_, k) =>
        params[k] !== undefined ? String(params[k]) : `{${k}}`,
      );
    },
    [locale],
  );

  const value = useMemo<I18nValue>(() => ({ locale, setLocale, t }), [locale, setLocale, t]);

  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

// useT is the hook every page calls. Throws if used outside the
// provider so a missing wrapper surfaces immediately in dev.
export function useT(): I18nValue {
  const ctx = useContext(I18nContext);
  if (!ctx) throw new Error("useT must be used within I18nProvider");
  return ctx;
}
