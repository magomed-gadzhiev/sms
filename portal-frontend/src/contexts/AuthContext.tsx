import { createContext, useContext, useState, useEffect, useCallback, type ReactNode } from 'react';
import { authApi, profileApi, ApiError, type ProfileData } from '../api/client';

type UserRole = 'client' | 'admin' | 'superadmin';

interface AuthState {
  user: ProfileData | null;
  role: UserRole;
  isAuthenticated: boolean;
  loading: boolean;
  isAdmin: boolean;
  login: (email: string, password: string) => Promise<LoginResult>;
  login2fa: (loginTicket: string, totpCode: string) => Promise<UserRole>;
  refreshUser: () => Promise<void>;
  logout: () => Promise<void>;
}

interface LoginResult {
  requires2fa: boolean;
  loginTicket?: string;
  role?: UserRole;
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
    return { requires2fa: false, role: (profile.role as UserRole) || 'client' };
  }, []);

  const login2fa = useCallback(async (loginTicket: string, totpCode: string): Promise<UserRole> => {
    await authApi.login2fa(loginTicket, totpCode);
    const profile = await profileApi.get();
    setUser(profile);
    return (profile.role as UserRole) || 'client';
  }, []);

  const refreshUser = useCallback(async () => {
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

  const role: UserRole = (user?.role as UserRole) || 'client';
  const isAdmin = role === 'admin' || role === 'superadmin';

  return (
    <AuthContext.Provider
      value={{ user, role, isAuthenticated: !!user, loading, isAdmin, login, login2fa, refreshUser, logout }}
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
