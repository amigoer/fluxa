import { useCallback, useEffect, useState } from "react"
import { api } from "@/lib/api"

// Shared data-fetching pattern behind every page: GET on mount, expose a
// refetch for after a mutation. Deliberately minimal -- no caching layer,
// no request de-duplication -- because nothing in this admin console
// needs that; a plain fetch-on-mount is enough.
//
// A null path skips the request entirely. Half the console's endpoints
// are permission-gated, and firing them for a member who will only get a
// 403 back is both noise in the log and a spurious error state in the UI.
export function useApiQuery<T>(path: string | null, deps: unknown[] = []) {
  const [data, setData] = useState<T | undefined>(undefined)
  const [loading, setLoading] = useState(path !== null)
  const [error, setError] = useState<unknown>(undefined)

  const refetch = useCallback(() => {
    if (path === null) {
      setData(undefined)
      setLoading(false)
      return
    }
    setLoading(true)
    api
      .get<T>(path)
      .then((res) => {
        setData(res)
        setError(undefined)
      })
      .catch((err) => setError(err))
      .finally(() => setLoading(false))
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [path, ...deps])

  useEffect(() => {
    refetch()
  }, [refetch])

  return { data, loading, error, refetch }
}
