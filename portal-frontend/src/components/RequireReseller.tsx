import { Navigate } from 'react-router-dom';
import { useAuth } from '../contexts/AuthContext';

interface RequireResellerProps {
  children: React.ReactNode;
}

export function RequireReseller({ children }: RequireResellerProps) {
  const { isAuthenticated, user, loading } = useAuth();

  if (loading) return <div className="p-8 text-center text-gray-400">Loading...</div>;
  if (!isAuthenticated) return <Navigate to="/login" replace />;
  if (!user?.is_reseller) return <Navigate to="/dashboard" replace />;

  return <>{children}</>;
}
