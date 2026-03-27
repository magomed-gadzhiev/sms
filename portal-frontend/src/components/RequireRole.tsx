import { Navigate } from 'react-router-dom';
import { useAuth } from '../contexts/AuthContext';

interface RequireRoleProps {
  role: 'admin' | 'superadmin';
  children: React.ReactNode;
}

export function RequireRole({ role, children }: RequireRoleProps) {
  const { isAuthenticated, role: userRole, loading } = useAuth();

  if (loading) return <div className="p-8 text-center text-gray-400">Loading...</div>;
  if (!isAuthenticated) return <Navigate to="/login" replace />;

  const hasAccess = role === 'admin'
    ? userRole === 'admin' || userRole === 'superadmin'
    : userRole === 'superadmin';

  if (!hasAccess) return <Navigate to="/dashboard" replace />;

  return <>{children}</>;
}
