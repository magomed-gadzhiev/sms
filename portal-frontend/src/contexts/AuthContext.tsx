import { createContext, useContext, useState, useEffect, useCallback, type ReactNode } from 'react';
import { authApi, profileApi, ApiError, type ProfileData } from '../api/client';

export type UserRole = 'client' | 'admin' | 'superadmin';

interface Permission {
  resource: string;
  action: string;
}

interface AuthState {
  user: ProfileData | null;
  role: UserRole;
  isAuthenticated: boolean;
  loading: boolean;
  isAdmin: boolean;
  permissions: Permission[];
  hasPermission: (resource: string, action: string) => boolean;
  login: (email: string, password: string) => Promise<LoginResult>;
  login2fa: (loginTicket: string, totpCode: string) => Promise<UserRole>;
  refreshUser: () => Promise<ProfileData>;
  logout: () => Promise<void>;
}

interface LoginResult {
  requires2fa: boolean;
  loginTicket?: string;
  role?: UserRole;
}

const AuthContext = createContext<AuthState | null>(null);

async function fetchPermissions(): Promise<Permission[]> {
  try {
    const csrfToken = document.cookie.match(new RegExp('(^| )csrf_token=([^;]+)'))?.[2] ?? null;
    const res = await fetch('/admin/v1/permissions', {
      credentials: 'include',
      headers: {
        'Content-Type': 'application/json',
        ...(csrfToken ? { 'X-CSRF-Token': csrfToken } : {}),
      },
    });
    if (!res.ok) return [];
    const data = await res.json();
    return (data.permissions || []).map((p: { resource: string; action: string }) => ({
      resource: p.resource,
      action: p.action,
    }));
  } catch {
    return [];
  }
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<ProfileData | null>(null);
  const [loading, setLoading] = useState(true);
  const [permissions, setPermissions] = useState<Permission[]>([]);

  const loadPermissions = useCallback(async (profile: ProfileData) => {
    const profileRole = (profile.role as UserRole) || 'client';
    if (profileRole === 'admin' || profileRole === 'superadmin') {
      const perms = await fetchPermissions();
      setPermissions(perms);
    } else {
      setPermissions([]);
    }
  }, []);

  useEffect(() => {
    profileApi
      .get()
      .then(async (profile) => {
        setUser(profile);
        await loadPermissions(profile);
      })
      .catch(() => setUser(null))
      .finally(() => setLoading(false));
  }, [loadPermissions]);

  const login = useCallback(async (email: string, password: string): Promise<LoginResult> => {
    const res = await authApi.login(email, password);
    if (res.requires_2fa) {
      return { requires2fa: true, loginTicket: res.login_ticket };
    }
    const profile = await profileApi.get();
    setUser(profile);
    await loadPermissions(profile);
    return { requires2fa: false, role: (profile.role as UserRole) || 'client' };
  }, [loadPermissions]);

  const login2fa = useCallback(async (loginTicket: string, totpCode: string): Promise<UserRole> => {
    await authApi.login2fa(loginTicket, totpCode);
    const profile = await profileApi.get();
    setUser(profile);
    await loadPermissions(profile);
    return (profile.role as UserRole) || 'client';
  }, [loadPermissions]);

  const refreshUser = useCallback(async () => {
    const profile = await profileApi.get();
    setUser(profile);
    await loadPermissions(profile);
    return profile;
  }, [loadPermissions]);

  const logout = useCallback(async () => {
    try {
      await authApi.logout();
    } catch (e) {
      if (!(e instanceof ApiError && e.status === 401)) throw e;
    } finally {
      setUser(null);
      setPermissions([]);
    }
  }, []);

  const role: UserRole = (user?.role as UserRole) || 'client';
  const isAdmin = role === 'admin' || role === 'superadmin';

  const hasPermission = useCallback((resource: string, action: string): boolean => {
    if (role === 'superadmin') return true;
    return permissions.some(p => p.resource === resource && p.action === action);
  }, [role, permissions]);

  return (
    <AuthContext.Provider
      value={{ user, role, isAuthenticated: !!user, loading, isAdmin, permissions, hasPermission, login, login2fa, refreshUser, logout }}
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
