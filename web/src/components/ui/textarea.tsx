// Textarea — shadcn-style multiline input. Used by the provider form
// for free-form key/value blocks (deployments, headers) where one
// line per entry beats trying to wedge a key-value editor into the
// dialog. Visually matches Input so the form stays cohesive.

import * as React from "react";
import { cn } from "@/lib/utils";

export type TextareaProps = React.TextareaHTMLAttributes<HTMLTextAreaElement>;

export const Textarea = React.forwardRef<HTMLTextAreaElement, TextareaProps>(
  ({ className, ...props }, ref) => (
    <textarea
      ref={ref}
      className={cn(
        "flex min-h-[80px] w-full rounded-md border border-input bg-transparent px-3 py-2 text-sm shadow-sm transition-colors placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50 font-mono",
        className,
      )}
      {...props}
    />
  ),
);
Textarea.displayName = "Textarea";
