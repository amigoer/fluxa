import * as React from "react";
import { cn } from "@/lib/utils";

// Separator is a thin horizontal/vertical divider used to break sidebar
// sections, card rows, and form groups apart without the visual weight
// of a full <hr>. It is intentionally a plain div so it does not pull
// in @radix-ui/react-separator just for one rule.
export interface SeparatorProps extends React.HTMLAttributes<HTMLDivElement> {
  orientation?: "horizontal" | "vertical";
}

export const Separator = React.forwardRef<HTMLDivElement, SeparatorProps>(
  ({ className, orientation = "horizontal", ...props }, ref) => (
    <div
      ref={ref}
      role="separator"
      aria-orientation={orientation}
      className={cn(
        "shrink-0 bg-border",
        orientation === "horizontal" ? "h-px w-full" : "h-full w-px",
        className,
      )}
      {...props}
    />
  ),
);
Separator.displayName = "Separator";
