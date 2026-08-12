import { useCallback, useEffect, useState } from "react";

import { ApiError } from "@/lib/api";

type Resource<T> = {
  data: T | null;
  error: string | null;
  loading: boolean;
  /** Re-runs the loader; call after any mutation. */
  reload: () => void;
};

/**
 * Minimal read-and-refresh hook. Pages need "fetch on mount, show a
 * skeleton, show an error, refetch after a write" and nothing more, so
 * this stays deliberately smaller than a caching data layer.
 *
 * `deps` behaves like a useEffect dependency list: change it and the
 * loader re-runs (that is how the log filters drive refetching).
 */
export function useResource<T>(loader: () => Promise<T>, deps: unknown[] = []): Resource<T> {
  const [data, setData] = useState<T | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [nonce, setNonce] = useState(0);

  const reload = useCallback(() => setNonce((n) => n + 1), []);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    loader()
      .then((result) => {
        if (cancelled) return;
        setData(result);
        setError(null);
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        // 401s are handled globally by AuthProvider; showing "session
        // expired" inside the page as well would just be noise.
        if (err instanceof ApiError && err.status === 401) return;
        setError(err instanceof Error ? err.message : String(err));
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [nonce, ...deps]);

  return { data, error, loading, reload };
}
