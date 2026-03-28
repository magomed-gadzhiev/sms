import { type ReactNode } from 'react';
import { useAuth } from '../../contexts/AuthContext';

interface RequirePermissionProps {
  resource: string;
  action: string;
  children: ReactNode;
  fallback?: ReactNode;
}

export function RequirePermission({ resource, action, children, fallback = null }: RequirePermissionProps) {
  const { hasPermission } = useAuth();
  if (!hasPermission(resource, action)) return <>{fallback}</>;
  return <>{children}</>;
}
