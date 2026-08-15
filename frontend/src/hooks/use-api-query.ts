import { useCallback, useEffect, useState } from "react"
import { api } from "@/lib/api"

// Shared data-fetching pattern behind every page: GET on mount, expose a
// refetch for after a mutation. Deliberately minimal -- no caching layer,
// no request de-duplication -- because nothing in this admin console
// needs that; a plain fetch-on-mount is enough.
export function useApiQuery<T>(path: string, deps: unknown[] = []) {
  const [data, setData] = useState<T | undefined>(undefined)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<unknown>(undefined)

  const refetch = useCallback(() => {
    setLoading(true)
    api
      .get<T>(path)
      .then((res) => setData(res))
      .catch((err) => setError(err))
      .finally(() => setLoading(false))
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [path, ...deps])

  useEffect(() => {
    refetch()
  }, [refetch])

  return { data, loading, error, refetch }
}
