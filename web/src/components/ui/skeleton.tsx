import * as React from "react";
import { cn } from "@/lib/utils";

// Skeleton is a placeholder block used while data is loading. It is a
// pulsing muted rectangle — pages compose several of them to mirror the
// shape of the content that will eventually replace it, so the layout
// does not jump on first paint.
export const Skeleton = React.forwardRef<
  HTMLDivElement,
  React.HTMLAttributes<HTMLDivElement>
>(({ className, ...props }, ref) => (
  <div
    ref={ref}
    className={cn("animate-pulse rounded-md bg-muted", className)}
    {...props}
  />
));
Skeleton.displayName = "Skeleton";
