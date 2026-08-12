import { cn } from "@/lib/utils";

/**
 * The small pill-shaped switch used by the header controls. Extracted so
 * the language and theme toggles cannot drift apart visually — they sit
 * next to each other and any difference in height, radius or padding is
 * immediately obvious.
 */
export function SegmentedGroup({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}) {
  return (
    <div
      role="radiogroup"
      aria-label={label}
      className="border-border bg-muted/50 inline-flex h-8 items-center gap-0.5 rounded-md border p-0.5"
    >
      {children}
    </div>
  );
}

export function SegmentedItem({
  checked,
  onSelect,
  label,
  className,
  children,
}: {
  checked: boolean;
  onSelect: () => void;
  /** Accessible name; also the tooltip, since some items are icon-only. */
  label: string;
  className?: string;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      role="radio"
      aria-checked={checked}
      aria-label={label}
      title={label}
      onClick={onSelect}
      className={cn(
        "flex h-7 items-center justify-center rounded-sm text-xs font-medium transition-colors",
        checked
          ? "bg-background text-foreground shadow-sm"
          : "text-muted-foreground hover:text-foreground",
        className,
      )}
    >
      {children}
    </button>
  );
}
