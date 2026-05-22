import { Navigate, useLocation } from 'react-router-dom';

export function DocsPage() {
  const prefix = useLocation().pathname.startsWith('/en') ? '/en' : '';
  return <Navigate to={`${prefix}/docs/getting-started`} replace />;
}
