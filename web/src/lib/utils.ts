import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

// cn merges conditional Tailwind class lists while letting later
// classes win on conflict. Every shadcn-style component uses this.
export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}
