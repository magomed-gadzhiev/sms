import { Outlet } from 'react-router-dom';
import { Sidebar, type NavItem } from './Sidebar';
import { SkipLink } from '../SkipLink';
import { useAuth } from '../../contexts/AuthContext';

const USER_NAV: NavItem[] = [
  { path: '/dashboard', label: 'Dashboard' },
  { path: '/messages', label: 'Messages' },
  { path: '/providers', label: 'Providers' },
  { path: '/api-keys', label: 'API Keys' },
  { path: '/webhooks', label: 'Webhooks' },
  { path: '/analytics', label: 'Analytics' },
  { path: '/sub-accounts', label: 'Sub-accounts' },
  { path: '/profile', label: 'Profile' },
  { path: '/audit-log', label: 'Audit Log' },
];

export function UserLayout() {
  const { user, logout } = useAuth();

  return (
    <div className="flex min-h-screen">
      <SkipLink targetId="main-content" />
      <Sidebar
        title="SMS Portal"
        items={USER_NAV}
        footer={
          <div>
            <div className="text-sm text-gray-600 truncate mb-2">{user?.email}</div>
            <button
              onClick={logout}
              className="text-sm text-gray-500 hover:text-gray-700"
            >
              Logout
            </button>
          </div>
        }
      />
      <main id="main-content" className="flex-1 p-6 bg-gray-50/50 overflow-auto">
        <Outlet />
      </main>
    </div>
  );
}
