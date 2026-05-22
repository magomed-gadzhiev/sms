import { useEffect } from 'react';
import { Navigate } from 'react-router-dom';
import { useAuth } from '../contexts/AuthContext';
import { useToast } from './ui/Toast';

interface RequireResellerProps {
  children: React.ReactNode;
}

export function RequireReseller({ children }: RequireResellerProps) {
  const { isAuthenticated, user, loading } = useAuth();
  const toast = useToast();

  const denied = !loading && isAuthenticated && !user?.is_reseller;

  useEffect(() => {
    if (denied) {
      toast.error('Доступ запрещён: раздел доступен только агрегаторам');
    }
  }, [denied, toast]);

  if (loading) return <div className="p-8 text-center text-gray-400">Загрузка...</div>;
  if (!isAuthenticated) return <Navigate to="/login" replace />;
  if (!user?.is_reseller) return <Navigate to="/dashboard" replace />;

  return <>{children}</>;
}
