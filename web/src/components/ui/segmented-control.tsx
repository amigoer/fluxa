import * as React from "react";
import { cn } from "@/lib/utils";

// SegmentedControl is a pill-style group of buttons for picking exactly
// one value from a short list. It is the project-wide replacement for
// the browser-native <select> when the option count is small enough
// that a dropdown would be more clicks than buttons.
//
// Generic over T so the value/onChange pair stays type-safe per
// callsite (e.g. SegmentedControl<"all" | "stream" | "nostream">).
export interface SegmentedOption<T extends string> {
  label: string;
  value: T;
  icon?: React.ReactNode;
}

interface SegmentedControlProps<T extends string> {
  options: SegmentedOption<T>[];
  value: T;
  onChange: (val: T) => void;
  className?: string;
  // size controls horizontal padding; "sm" matches filter bars, the
  // default matches Settings-page rows.
  size?: "default" | "sm";
}

export function SegmentedControl<T extends string>({
  options,
  value,
  onChange,
  className,
  size = "default",
}: SegmentedControlProps<T>) {
  return (
    <div
      className={cn(
        "flex w-full sm:w-auto items-center p-1 rounded-xl bg-accent/50 text-muted-foreground border border-border/40",
        className,
      )}
    >
      {options.map((opt) => {
        const isActive = value === opt.value;
        return (
          <button
            key={opt.value}
            type="button"
            onClick={() => onChange(opt.value)}
            className={cn(
              "flex flex-1 sm:flex-initial items-center justify-center whitespace-nowrap rounded-lg font-medium transition-all duration-300",
              size === "sm" ? "px-3 py-1 text-[12px]" : "px-4 py-1.5 text-[13px]",
              isActive
                ? "bg-background text-foreground shadow-sm ring-1 ring-border/20 scale-[0.98] sm:scale-100"
                : "hover:text-foreground opacity-80 hover:opacity-100 hover:bg-background/40",
            )}
          >
            {opt.icon && <span className="mr-1.5 shrink-0">{opt.icon}</span>}
            {opt.label}
          </button>
        );
      })}
    </div>
  );
}
