// Shared formatters. Kept in one place so a token count renders the same
// way on the overview, the key list and the log table.
//
// The active locale is module state rather than a parameter on every
// call: the console renders one locale at a time, and threading it
// through ~30 call sites would add noise without adding a capability.
// I18nProvider owns it and calls setFormatLocale on every change.

let activeLocale = "en";

export function setFormatLocale(locale: string) {
  activeLocale = locale;
}

export const formatNumber = (value: number) =>
  new Intl.NumberFormat(activeLocale).format(value ?? 0);

export const formatUSD = (value: number) =>
  new Intl.NumberFormat(activeLocale, {
    style: "currency",
    currency: "USD",
    minimumFractionDigits: 2,
    maximumFractionDigits: value > 0 && value < 0.01 ? 4 : 2,
  }).format(value ?? 0);

export const formatMs = (value: number) => (value > 0 ? `${formatNumber(value)} ms` : "—");

export function formatDateTime(iso?: string | null) {
  if (!iso) return "—";
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return "—";
  return date.toLocaleString(activeLocale, {
    year: "numeric",
    month: "short",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
}

/** "3 min ago" style stamps for log rows, where absolute time is noise. */
export function formatRelative(iso?: string | null) {
  if (!iso) return "—";
  const then = new Date(iso).getTime();
  if (Number.isNaN(then)) return "—";
  const seconds = Math.round((Date.now() - then) / 1000);
  // Intl.RelativeTimeFormat keeps this idiomatic per locale instead of
  // hardcoding an English "5m ago" shape.
  const rtf = new Intl.RelativeTimeFormat(activeLocale, { numeric: "auto", style: "narrow" });
  if (seconds < 60) return rtf.format(-seconds, "second");
  if (seconds < 3600) return rtf.format(-Math.round(seconds / 60), "minute");
  if (seconds < 86400) return rtf.format(-Math.round(seconds / 3600), "hour");
  return rtf.format(-Math.round(seconds / 86400), "day");
}

/** Splits a comma/newline separated textarea into a clean string list. */
export const parseList = (raw: string) =>
  raw
    .split(/[\n,]/)
    .map((item) => item.trim())
    .filter(Boolean);

/** Best-effort pretty printer for the request/response bodies in the log detail. */
export function prettyJSON(raw: string) {
  if (!raw) return "";
  try {
    return JSON.stringify(JSON.parse(raw), null, 2);
  } catch {
    return raw;
  }
}
