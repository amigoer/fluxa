import { createContext, use, useCallback, useEffect, useMemo, useState } from "react";

import { api, getToken, setToken, UNAUTHORIZED_EVENT, type User } from "@/lib/api";

type AuthState = {
  user: User | null;
  /** True until the stored token has been validated against /admin/auth/me. */
  loading: boolean;
  login: (username: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
  refresh: () => Promise<void>;
};

const AuthContext = createContext<AuthState | null>(null);

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);

  const refresh = useCallback(async () => {
    if (!getToken()) {
      setUser(null);
      setLoading(false);
      return;
    }
    try {
      setUser(await api.me());
    } catch {
      // Expired or revoked token; the request layer already cleared it.
      setUser(null);
    } finally {
      setLoading(false);
    }
  }, []);

  // Validate whatever token survived the last session before rendering
  // anything behind the auth wall.
  useEffect(() => {
    void refresh();
  }, [refresh]);

  // Any 401 from anywhere in the app drops us back to signed-out state.
  useEffect(() => {
    const onUnauthorized = () => setUser(null);
    window.addEventListener(UNAUTHORIZED_EVENT, onUnauthorized);
    return () => window.removeEventListener(UNAUTHORIZED_EVENT, onUnauthorized);
  }, []);

  const login = useCallback(async (username: string, password: string) => {
    const result = await api.login(username, password);
    setToken(result.token);
    setUser(result.user);
  }, []);

  const logout = useCallback(async () => {
    try {
      await api.logout();
    } catch {
      // Revoking server-side is best effort; the local token goes either way.
    }
    setToken(null);
    setUser(null);
  }, []);

  const value = useMemo(
    () => ({ user, loading, login, logout, refresh }),
    [user, loading, login, logout, refresh],
  );

  return <AuthContext value={value}>{children}</AuthContext>;
}

export function useAuth() {
  const context = use(AuthContext);
  if (!context) throw new Error("useAuth must be used inside an AuthProvider");
  return context;
}
