import { Outlet } from 'react-router-dom';
import { Sidebar, type NavItem } from '../../components/layout/Sidebar';
import { useAuth } from '../../contexts/AuthContext';

const ADMIN_NAV: NavItem[] = [
  { path: '/admin/clients', label: 'Clients' },
  { path: '/admin/providers', label: 'Providers' },
  { path: '/admin/routes', label: 'Routes' },
  { path: '/admin/billing', label: 'Billing' },
  { path: '/admin/monitoring', label: 'Monitoring' },
  { path: '/admin/analytics', label: 'Analytics' },
  { path: '/admin/templates', label: 'Templates' },
  { path: '/admin/webhooks', label: 'Webhooks' },
  { path: '/admin/hlr', label: 'HLR' },
  { path: '/admin/countries', label: 'Countries' },
  { path: '/admin/audit', label: 'Audit Log' },
];

export function AdminLayout() {
  const { user, logout } = useAuth();

  return (
    <div className="flex min-h-screen">
      <Sidebar
        title="SMS Admin"
        items={ADMIN_NAV}
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
      <main className="flex-1 p-6 bg-gray-50/50 overflow-auto">
        <Outlet />
      </main>
    </div>
  );
}
