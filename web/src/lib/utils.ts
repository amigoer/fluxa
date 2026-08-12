import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

// cn merges conditional class lists and lets a later Tailwind utility win
// over an earlier conflicting one, which is what makes component-level
// className overrides predictable.
export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}
