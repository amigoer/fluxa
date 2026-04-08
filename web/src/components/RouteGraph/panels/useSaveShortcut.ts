// useSaveShortcut — small hook that wires Cmd/Ctrl+S to a side
// panel's save callback. Mounted by RegexRoutePanel,
// VirtualModelPanel and ProviderCreatePanel so the operator can
// hit the keyboard shortcut from anywhere on the page (the
// listener lives on `window`, not on the input that is currently
// focused).
//
// We capture the latest save function via a ref so we can attach
// the keydown listener exactly once per mount — without the ref,
// the listener would close over the very first render's `save`
// and miss every subsequent form edit. The hook also respects an
// `enabled` flag so the panel can disable the shortcut while a
// previous save is still in flight, preventing double-submits.

import { useEffect, useRef } from "react";

export function useSaveShortcut(save: () => void | Promise<void>, enabled = true) {
  const saveRef = useRef(save);
  saveRef.current = save;
  const enabledRef = useRef(enabled);
  enabledRef.current = enabled;

  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      // Match Cmd+S on macOS and Ctrl+S elsewhere. We check both
      // lowercase and uppercase `s` so the shortcut still fires
      // when the operator is holding shift (some keyboards report
      // 'S' instead of 's' under shift).
      const isSaveCombo =
        (e.metaKey || e.ctrlKey) && !e.altKey && (e.key === "s" || e.key === "S");
      if (!isSaveCombo) return;
      // preventDefault stops the browser's "Save Page As" dialog
      // from popping up over our editor.
      e.preventDefault();
      if (!enabledRef.current) return;
      void saveRef.current();
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);
}

// Display string for the save shortcut hint, picked once on import
// based on the user's platform. macOS shows ⌘S; everyone else gets
// Ctrl+S. We use the deprecated `navigator.platform` because it's
// the simplest reliable signal here — userAgent parsing is more
// fragile and a misdetected hint is purely cosmetic.
export const SAVE_SHORTCUT_HINT =
  typeof navigator !== "undefined" && /Mac|iPhone|iPad/i.test(navigator.platform)
    ? "⌘S"
    : "Ctrl+S";
