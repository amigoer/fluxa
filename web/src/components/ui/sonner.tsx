// Custom-themed toast layer built on top of `sonner`. We intentionally
// do NOT use the default sonner skin — instead each variant gets a
// Fluxa-branded card (rounded-xl, thin border, soft shadow, frosted
// background) and its own accent color:
//   success → emerald green
//   warning → amber yellow
//   error   → rose red
//   info    → sky blue
//
// The wrapper below (`toast`) is what pages should import: it forwards
// to sonner's `toast.success/.error/.warning/.info` but also pins the
// correct `className` so the shared classes above apply every time.

import { Toaster as SonnerToaster, toast as sonnerToast } from "sonner";
import type { ExternalToast } from "sonner";
import { CheckCircle2, AlertTriangle, XCircle, Info } from "lucide-react";
import * as React from "react";

// Shared card shell — every toast shares the same geometry / typography
// so only the accent color changes between variants.
const BASE_TOAST =
  "group pointer-events-auto flex w-full items-start gap-3 overflow-hidden " +
  "rounded-xl border px-4 py-3 text-sm font-medium shadow-lg " +
  "backdrop-blur supports-[backdrop-filter]:bg-opacity-90 " +
  "transition-all";

const VARIANT_CLASS = {
  success:
    "border-emerald-200 bg-emerald-50/95 text-emerald-900 " +
    "dark:border-emerald-500/30 dark:bg-emerald-950/80 dark:text-emerald-100",
  warning:
    "border-amber-200 bg-amber-50/95 text-amber-900 " +
    "dark:border-amber-500/30 dark:bg-amber-950/80 dark:text-amber-100",
  error:
    "border-rose-200 bg-rose-50/95 text-rose-900 " +
    "dark:border-rose-500/30 dark:bg-rose-950/80 dark:text-rose-100",
  info:
    "border-sky-200 bg-sky-50/95 text-sky-900 " +
    "dark:border-sky-500/30 dark:bg-sky-950/80 dark:text-sky-100",
} as const;

const ICON_CLASS = {
  success: "text-emerald-600 dark:text-emerald-300",
  warning: "text-amber-600 dark:text-amber-300",
  error: "text-rose-600 dark:text-rose-300",
  info: "text-sky-600 dark:text-sky-300",
} as const;

// Toaster component — mount once at the app root. We disable the
// built-in color/theme engine (richColors / theme="light") by hard-
// coding `unstyled` on each toast and providing our own classes via
// the toast helper below.
export function Toaster() {
  return (
    <SonnerToaster
      position="top-center"
      expand={false}
      offset={20}
      gap={10}
      duration={3200}
      visibleToasts={4}
      toastOptions={{
        // Base wrapper class — variants layer on top via the helper.
        unstyled: true,
        classNames: {
          toast: BASE_TOAST,
          title: "leading-5",
          description: "mt-0.5 text-xs font-normal opacity-80",
          closeButton:
            "absolute right-2 top-2 rounded-md p-0.5 opacity-60 hover:opacity-100 " +
            "hover:bg-black/5 dark:hover:bg-white/10",
        },
      }}
    />
  );
}

type Variant = "success" | "warning" | "error" | "info";

const ICONS: Record<Variant, React.ReactNode> = {
  success: <CheckCircle2 className={`h-5 w-5 shrink-0 ${ICON_CLASS.success}`} />,
  warning: <AlertTriangle className={`h-5 w-5 shrink-0 ${ICON_CLASS.warning}`} />,
  error: <XCircle className={`h-5 w-5 shrink-0 ${ICON_CLASS.error}`} />,
  info: <Info className={`h-5 w-5 shrink-0 ${ICON_CLASS.info}`} />,
};

// Build the options passed to `sonnerToast.*`. We inject the variant
// icon and tack the accent class onto the base card so every toast
// looks the same except for its color.
function build(variant: Variant, data?: ExternalToast): ExternalToast {
  return {
    ...data,
    icon: ICONS[variant],
    className: [BASE_TOAST, VARIANT_CLASS[variant], data?.className ?? ""].join(" "),
  };
}

// Public API — matches the shape of `sonner.toast.*` but wired into
// our Fluxa-themed card. Pages should import this instead of sonner.
export const toast = {
  success: (message: string, data?: ExternalToast) =>
    sonnerToast.success(message, build("success", data)),
  warning: (message: string, data?: ExternalToast) =>
    sonnerToast.warning(message, build("warning", data)),
  error: (message: string, data?: ExternalToast) =>
    sonnerToast.error(message, build("error", data)),
  info: (message: string, data?: ExternalToast) =>
    sonnerToast.message(message, build("info", data)),
  dismiss: sonnerToast.dismiss,
};
