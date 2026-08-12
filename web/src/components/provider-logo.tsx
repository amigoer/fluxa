import { VENDOR_MARKS } from "@/lib/vendor-marks";
import { cn } from "@/lib/utils";

/**
 * The upstream's own mark, in a tile tinted with its brand colour.
 *
 * Recognition is the whole point — an operator scanning a list of twenty
 * providers finds "the Anthropic one" by its shape long before reading
 * the name. The mark stays single-colour and the brand hue is carried by
 * a low-alpha tile, which keeps ten vendors in a table from turning into
 * a sticker sheet.
 *
 * Anything unknown (a custom OpenAI-compatible upstream, a first-class
 * case here) gets a lettermark in a colour derived from its name, so it
 * still reads as a distinct thing rather than a blank.
 */
export function ProviderLogo({
  kind,
  name,
  className,
}: {
  /** Provider kind, e.g. "openai". Falls back to `name` when unset. */
  kind?: string;
  name: string;
  className?: string;
}) {
  const key = (kind || name).toLowerCase();
  const mark = VENDOR_MARKS[key];

  if (!mark) return <Lettermark name={name} className={className} />;

  return (
    <span
      data-vendor-mark
      className={cn(
        "flex size-7 shrink-0 items-center justify-center rounded-md border",
        className,
      )}
      style={
        {
          "--mark": mark.color,
          "--mark-dark": onDark(mark.color),
        } as React.CSSProperties
      }
      title={name}
    >
      <svg viewBox={mark.viewBox} fill="currentColor" aria-hidden className="size-[60%]">
        {mark.paths.map((d, i) => (
          <path key={i} d={d} fillRule="evenodd" clipRule="evenodd" />
        ))}
      </svg>
    </span>
  );
}

/** Deterministic two-letter tile for upstreams we have no mark for. */
function Lettermark({ name, className }: { name: string; className?: string }) {
  const initials = name.replace(/[^a-zA-Z0-9]/g, "").slice(0, 2).toUpperCase() || "?";
  // Hue from the name, so one provider is always the same colour and two
  // in the same list are unlikely to collide.
  let hash = 0;
  for (let i = 0; i < name.length; i++) hash = (hash * 31 + name.charCodeAt(i)) % 360;

  return (
    <span
      data-vendor-mark
      className={cn(
        "flex size-7 shrink-0 items-center justify-center rounded-md border text-[10px] font-semibold",
        className,
      )}
      style={
        {
          "--mark": `hsl(${hash} 55% 38%)`,
          "--mark-dark": `hsl(${hash} 70% 72%)`,
        } as React.CSSProperties
      }
      title={name}
    >
      {initials}
    </span>
  );
}

/**
 * A usable version of a brand colour on a near-black surface.
 *
 * Several vendors ship a black or near-black mark — Anthropic is
 * #191919, Ollama and xAI are pure black — which disappears entirely in
 * the dark theme. Anything below the luminance threshold gets lifted
 * towards white, keeping whatever hue it had.
 */
function onDark(hex: string): string {
  const value = hex.replace("#", "");
  const full =
    value.length === 3
      ? value.split("").map((c) => c + c).join("")
      : value.padEnd(6, "0").slice(0, 6);
  const channels = [0, 2, 4].map((i) => parseInt(full.slice(i, i + 2), 16) / 255);
  const linear = channels.map((c) => (c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4));
  const luminance = 0.2126 * linear[0] + 0.7152 * linear[1] + 0.0722 * linear[2];
  if (luminance >= 0.16) return hex;
  // Mixing in oklab keeps the hue rather than washing it grey.
  return `color-mix(in oklab, ${hex} 30%, hsl(0 0% 88%))`;
}
