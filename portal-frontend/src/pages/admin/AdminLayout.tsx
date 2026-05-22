import { Outlet } from 'react-router-dom';
import { AdminSidebar } from '../../components/layout/AdminSidebar';

export function AdminLayout() {
  return (
    <div className="flex min-h-screen">
      <AdminSidebar />
      <main className="flex-1 ml-14 p-6 bg-gray-50 dark:bg-slate-950/50 overflow-auto">
        <Outlet />
      </main>
    </div>
  );
}
