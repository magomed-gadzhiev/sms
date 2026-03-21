import { createContext, useContext, useState, useEffect, useCallback, type ReactNode } from 'react';
import { authApi, profileApi, ApiError, type ProfileData } from '../api/client';

interface AuthState {
  user: ProfileData | null;
  isAuthenticated: boolean;
  loading: boolean;
  login: (email: string, password: string) => Promise<LoginResult>;
  login2fa: (loginTicket: string, totpCode: string) => Promise<void>;
  logout: () => Promise<void>;
}

interface LoginResult {
  requires2fa: boolean;
  loginTicket?: string;
}

const AuthContext = createContext<AuthState | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<ProfileData | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    profileApi
      .get()
      .then(setUser)
      .catch(() => setUser(null))
      .finally(() => setLoading(false));
  }, []);

  const login = useCallback(async (email: string, password: string): Promise<LoginResult> => {
    const res = await authApi.login(email, password);
    if (res.requires_2fa) {
      return { requires2fa: true, loginTicket: res.login_ticket };
    }
    const profile = await profileApi.get();
    setUser(profile);
    return { requires2fa: false };
  }, []);

  const login2fa = useCallback(async (loginTicket: string, totpCode: string) => {
    await authApi.login2fa(loginTicket, totpCode);
    const profile = await profileApi.get();
    setUser(profile);
  }, []);

  const logout = useCallback(async () => {
    try {
      await authApi.logout();
    } catch (e) {
      if (!(e instanceof ApiError && e.status === 401)) throw e;
    }
    setUser(null);
  }, []);

  return (
    <AuthContext.Provider
      value={{ user, isAuthenticated: !!user, loading, login, login2fa, logout }}
    >
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth(): AuthState {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error('useAuth must be used within AuthProvider');
  return ctx;
}
