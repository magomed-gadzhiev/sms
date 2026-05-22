import { useEffect } from 'react';
import { Navigate } from 'react-router-dom';
import { useAuth } from '../contexts/AuthContext';
import { useToast } from './ui/Toast';

interface RequireRoleProps {
  role: 'admin' | 'superadmin';
  children: React.ReactNode;
}

export function RequireRole({ role, children }: RequireRoleProps) {
  const { isAuthenticated, role: userRole, loading } = useAuth();
  const toast = useToast();

  const hasAccess = role === 'admin'
    ? userRole === 'admin' || userRole === 'superadmin'
    : userRole === 'superadmin';

  const denied = !loading && isAuthenticated && !hasAccess;

  useEffect(() => {
    if (denied) {
      toast.error(`Доступ запрещён: требуется роль «${role}»`);
    }
  }, [denied, role, toast]);

  if (loading) return <div className="p-8 text-center text-gray-400">Загрузка...</div>;
  if (!isAuthenticated) return <Navigate to="/login" replace />;
  if (!hasAccess) return <Navigate to="/dashboard" replace />;

  return <>{children}</>;
}
