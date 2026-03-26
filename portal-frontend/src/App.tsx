import { Routes, Route, Navigate, Link, Outlet, useLocation } from 'react-router-dom';
import { useAuth } from './contexts/AuthContext';
import { SkipLink } from './components/SkipLink';
import { LoginPage } from './pages/auth/LoginPage';
import { PasswordResetRequestPage } from './pages/auth/PasswordResetRequestPage';
import { PasswordResetPage } from './pages/auth/PasswordResetPage';
import { ProfilePage } from './pages/profile/ProfilePage';
import { DashboardPage } from './pages/dashboard/DashboardPage';
import { MessagesPage } from './pages/messages/MessagesPage';
import { APIKeysPage } from './pages/api-keys/APIKeysPage';
import { AnalyticsPage } from './pages/analytics/AnalyticsPage';
import { WebhooksPage } from './pages/webhooks/WebhooksPage';
import { SubAccountsListPage } from './pages/sub-accounts/SubAccountsListPage';
import { SubAccountDetailPage } from './pages/sub-accounts/SubAccountDetailPage';
import { AuditLogPage } from './pages/audit/AuditLogPage';

function RequireAuth() {
  const { isAuthenticated, loading } = useAuth();
  const location = useLocation();

  if (loading) return <div>Loading...</div>;
  if (!isAuthenticated) return <Navigate to="/login" state={{ from: location }} replace />;
  return <Layout />;
}

const NAV_ITEMS = [
  { path: '/dashboard', label: 'Dashboard' },
  { path: '/messages', label: 'Messages' },
  { path: '/api-keys', label: 'API Keys' },
  { path: '/webhooks', label: 'Webhooks' },
  { path: '/analytics', label: 'Analytics' },
  { path: '/sub-accounts', label: 'Sub-accounts' },
  { path: '/profile', label: 'Profile' },
  { path: '/audit-log', label: 'Audit Log' },
];

function Layout() {
  const { user, logout } = useAuth();
  const location = useLocation();

  return (
    <div style={{ display: 'flex', minHeight: '100vh' }}>
      <SkipLink targetId="main-content" />
      <nav aria-label="Main navigation" style={{ width: 220, padding: 16, borderRight: '1px solid #ddd' }}>
        <h3 style={{ marginTop: 0 }}>SMS Portal</h3>
        <ul style={{ listStyle: 'none', padding: 0 }}>
          {NAV_ITEMS.map((item) => {
            const isActive = location.pathname === item.path;
            return (
              <li key={item.path} style={{ marginBottom: 8 }}>
                <Link
                  to={item.path}
                  aria-current={isActive ? 'page' : undefined}
                  style={{
                    fontWeight: isActive ? 'bold' : 'normal',
                    borderLeft: isActive ? '3px solid #1976d2' : undefined,
                    paddingLeft: isActive ? 8 : undefined,
                  }}
                >
                  {item.label}
                </Link>
              </li>
            );
          })}
        </ul>
        <hr />
        <div style={{ fontSize: 14 }}>{user?.email}</div>
        <button onClick={logout} style={{ marginTop: 8 }}>
          Logout
        </button>
      </nav>
      <main id="main-content" style={{ flex: 1, padding: 24 }}>
        <Outlet />
      </main>
    </div>
  );
}

export function App() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route path="/reset-password-request" element={<PasswordResetRequestPage />} />
      <Route path="/reset-password" element={<PasswordResetPage />} />

      <Route element={<RequireAuth />}>
        <Route path="/dashboard" element={<DashboardPage />} />
        <Route path="/messages" element={<MessagesPage />} />
        <Route path="/api-keys" element={<APIKeysPage />} />
        <Route path="/webhooks" element={<WebhooksPage />} />
        <Route path="/analytics" element={<AnalyticsPage />} />
        <Route path="/sub-accounts" element={<SubAccountsListPage />} />
        <Route path="/sub-accounts/:id" element={<SubAccountDetailPage />} />
        <Route path="/profile" element={<ProfilePage />} />
        <Route path="/audit-log" element={<AuditLogPage />} />
      </Route>

      <Route path="*" element={<Navigate to="/dashboard" replace />} />
    </Routes>
  );
}
