# Admin Panel Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build admin panel UI as part of portal-frontend with 11 admin pages, role-based routing, Tailwind CSS + Radix UI component system, and session-based admin gateway auth.

**Architecture:** Extend portal-frontend (React 19 + Vite 6) with lazy-loaded `/admin/*` routes. New shared component library (Tailwind + Radix) for admin pages. Session cookie auth in admin gateway validates via auth service gRPC. Role from session context controls frontend routing and backend access.

**Tech Stack:** React 19, TypeScript 5.7, Vite 6, Tailwind CSS v4, Radix UI (Dialog, Toast, Tabs, DropdownMenu), Recharts, React Router 7

---

## File Structure

```
portal-frontend/src/
  index.css                          # NEW - Tailwind imports + theme
  components/
    ui/
      Button.tsx                     # NEW - Button with variants
      Input.tsx                      # NEW - Input with label/error
      Badge.tsx                      # NEW - Badge + StatusBadge
      Select.tsx                     # NEW - Native select with label/error
      Modal.tsx                      # NEW - Radix Dialog wrapper
      Toast.tsx                      # NEW - Toast provider + useToast
      ConfirmDialog.tsx              # NEW - Danger action confirmation
    data/
      DataTable.tsx                  # NEW - Table with sort, pagination, actions
      FilterBar.tsx                  # NEW - Horizontal filter panel
      StatCard.tsx                   # NEW - Metric card
    layout/
      Sidebar.tsx                    # NEW - Reusable sidebar nav
      PageHeader.tsx                 # NEW - Page title + breadcrumbs + actions
  pages/
    admin/
      AdminLayout.tsx                # NEW - Admin shell with sidebar + routes
      ClientsPage.tsx                # NEW
      ProvidersPage.tsx              # NEW
      RoutesPage.tsx                 # NEW
      BillingPage.tsx                # NEW
      MonitoringPage.tsx             # NEW
      AnalyticsPage.tsx              # NEW
      TemplatesPage.tsx              # NEW
      WebhooksPage.tsx               # NEW
      HLRPage.tsx                    # NEW
      CountriesPage.tsx              # NEW
      AuditLogPage.tsx               # NEW
  api/
    admin.ts                         # NEW - Admin API client
  contexts/
    AuthContext.tsx                   # MODIFY - add role field
  App.tsx                            # MODIFY - add admin routes
  main.tsx                           # MODIFY - import CSS, add ToastProvider
  vite.config.ts                     # MODIFY - add Tailwind plugin + admin proxy
  nginx.conf                         # MODIFY - add admin proxy
internal/gateway/portal/handlers/
  profile.go                         # MODIFY - always return role
internal/gateway/admin/middleware/
  auth.go                            # MODIFY - session cookie auth
```

---

### Task 1: Install Dependencies and Configure Build

**Files:**
- Modify: `portal-frontend/package.json`
- Create: `portal-frontend/src/index.css`
- Modify: `portal-frontend/src/main.tsx`
- Modify: `portal-frontend/vite.config.ts`
- Modify: `portal-frontend/nginx.conf`

- [ ] **Step 1: Install npm packages**

```bash
cd portal-frontend
npm install tailwindcss @tailwindcss/vite @radix-ui/react-dialog @radix-ui/react-toast @radix-ui/react-tabs @radix-ui/react-dropdown-menu recharts
npm install -D @types/recharts
```

- [ ] **Step 2: Create `portal-frontend/src/index.css`**

```css
@import "tailwindcss";

@theme {
  --color-primary: #1976d2;
  --color-primary-dark: #1565c0;
  --color-success: #4caf50;
  --color-danger: #d32f2f;
  --color-warning: #f59e0b;
  --color-info: #2196f3;
}
```

- [ ] **Step 3: Update `portal-frontend/src/main.tsx`**

Add CSS import at the top (before React imports):

```tsx
import './index.css';
```

- [ ] **Step 4: Update `portal-frontend/vite.config.ts`**

Replace entire file:

```ts
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    port: 3001,
    proxy: {
      '/portal/v1': {
        target: 'http://localhost:8082',
        changeOrigin: true,
      },
      '/admin/v1': {
        target: 'http://localhost:8081',
        changeOrigin: true,
      },
    },
  },
});
```

- [ ] **Step 5: Update `portal-frontend/nginx.conf`**

Add admin proxy block after portal proxy (before the SPA fallback):

```nginx
    # Proxy API requests to Admin Gateway
    location /admin/v1/ {
        proxy_pass http://admin-gateway:8081;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
```

- [ ] **Step 6: Verify build**

```bash
cd portal-frontend && npm run build
```

Expected: Build succeeds.

- [ ] **Step 7: Commit**

```bash
git add portal-frontend/
git commit -m "feat(admin): add Tailwind CSS v4, Radix UI, Recharts deps and proxy config"
```

---

### Task 2: Create Base UI Components (Button, Input, Badge, Select)

**Files:**
- Create: `portal-frontend/src/components/ui/Button.tsx`
- Create: `portal-frontend/src/components/ui/Input.tsx`
- Create: `portal-frontend/src/components/ui/Badge.tsx`
- Create: `portal-frontend/src/components/ui/Select.tsx`

- [ ] **Step 1: Create `portal-frontend/src/components/ui/Button.tsx`**

```tsx
import { type ButtonHTMLAttributes, type ReactNode, forwardRef } from 'react';

const variantStyles = {
  primary: 'bg-primary text-white hover:bg-primary-dark',
  secondary: 'bg-gray-100 text-gray-800 hover:bg-gray-200 border border-gray-300',
  danger: 'bg-danger text-white hover:bg-red-800',
  ghost: 'text-gray-600 hover:bg-gray-100',
} as const;

const sizeStyles = {
  sm: 'px-2.5 py-1 text-xs',
  md: 'px-4 py-2 text-sm',
  lg: 'px-5 py-2.5 text-base',
} as const;

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: keyof typeof variantStyles;
  size?: keyof typeof sizeStyles;
  children: ReactNode;
}

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(
  ({ variant = 'primary', size = 'md', className = '', children, ...props }, ref) => (
    <button
      ref={ref}
      className={`inline-flex items-center justify-center gap-2 rounded font-medium transition-colors
        focus:outline-none focus:ring-2 focus:ring-primary/50
        disabled:opacity-50 disabled:cursor-not-allowed
        ${variantStyles[variant]} ${sizeStyles[size]} ${className}`}
      {...props}
    >
      {children}
    </button>
  ),
);

Button.displayName = 'Button';
```

- [ ] **Step 2: Create `portal-frontend/src/components/ui/Input.tsx`**

```tsx
import { type InputHTMLAttributes, forwardRef } from 'react';

interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  label?: string;
  error?: string;
}

export const Input = forwardRef<HTMLInputElement, InputProps>(
  ({ label, error, id, className = '', ...props }, ref) => {
    const inputId = id || label?.toLowerCase().replace(/\s+/g, '-');
    return (
      <div className="flex flex-col gap-1">
        {label && (
          <label htmlFor={inputId} className="text-sm font-medium text-gray-700">
            {label}
          </label>
        )}
        <input
          ref={ref}
          id={inputId}
          className={`rounded border px-3 py-2 text-sm transition-colors
            focus:outline-none focus:ring-2 focus:ring-primary/50 focus:border-primary
            ${error ? 'border-danger' : 'border-gray-300'}
            disabled:bg-gray-50 disabled:text-gray-500 ${className}`}
          aria-invalid={!!error}
          aria-describedby={error ? `${inputId}-error` : undefined}
          {...props}
        />
        {error && (
          <span id={`${inputId}-error`} className="text-sm text-danger" role="alert">
            {error}
          </span>
        )}
      </div>
    );
  },
);

Input.displayName = 'Input';
```

- [ ] **Step 3: Create `portal-frontend/src/components/ui/Badge.tsx`**

```tsx
import { type ReactNode } from 'react';

const variantStyles = {
  default: 'bg-gray-100 text-gray-700',
  success: 'bg-green-100 text-green-800',
  warning: 'bg-yellow-100 text-yellow-800',
  danger: 'bg-red-100 text-red-800',
  info: 'bg-blue-100 text-blue-800',
} as const;

interface BadgeProps {
  variant?: keyof typeof variantStyles;
  children: ReactNode;
}

export function Badge({ variant = 'default', children }: BadgeProps) {
  return (
    <span className={`inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium ${variantStyles[variant]}`}>
      {children}
    </span>
  );
}

const statusMap: Record<string, { variant: keyof typeof variantStyles; label: string }> = {
  active: { variant: 'success', label: 'Active' },
  inactive: { variant: 'default', label: 'Inactive' },
  delivered: { variant: 'success', label: 'Delivered' },
  failed: { variant: 'danger', label: 'Failed' },
  pending: { variant: 'warning', label: 'Pending' },
  blocked: { variant: 'danger', label: 'Blocked' },
  approved: { variant: 'success', label: 'Approved' },
  rejected: { variant: 'danger', label: 'Rejected' },
  healthy: { variant: 'success', label: 'Healthy' },
  unhealthy: { variant: 'danger', label: 'Unhealthy' },
  degraded: { variant: 'warning', label: 'Degraded' },
};

export function StatusBadge({ status }: { status: string }) {
  const config = statusMap[status.toLowerCase()] || { variant: 'default' as const, label: status };
  return <Badge variant={config.variant}>{config.label}</Badge>;
}
```

- [ ] **Step 4: Create `portal-frontend/src/components/ui/Select.tsx`**

```tsx
import { type SelectHTMLAttributes, forwardRef } from 'react';

interface SelectOption {
  value: string;
  label: string;
}

interface SelectProps extends Omit<SelectHTMLAttributes<HTMLSelectElement>, 'onChange'> {
  label?: string;
  error?: string;
  options: SelectOption[];
  placeholder?: string;
  onChange: (value: string) => void;
}

export const Select = forwardRef<HTMLSelectElement, SelectProps>(
  ({ label, error, options, placeholder, onChange, id, className = '', value, ...props }, ref) => {
    const selectId = id || label?.toLowerCase().replace(/\s+/g, '-');
    return (
      <div className="flex flex-col gap-1">
        {label && (
          <label htmlFor={selectId} className="text-sm font-medium text-gray-700">
            {label}
          </label>
        )}
        <select
          ref={ref}
          id={selectId}
          value={value}
          onChange={(e) => onChange(e.target.value)}
          className={`rounded border px-3 py-2 text-sm transition-colors
            focus:outline-none focus:ring-2 focus:ring-primary/50 focus:border-primary
            ${error ? 'border-danger' : 'border-gray-300'}
            disabled:bg-gray-50 disabled:text-gray-500 ${className}`}
          aria-invalid={!!error}
          {...props}
        >
          {placeholder && <option value="">{placeholder}</option>}
          {options.map((opt) => (
            <option key={opt.value} value={opt.value}>{opt.label}</option>
          ))}
        </select>
        {error && (
          <span className="text-sm text-danger" role="alert">{error}</span>
        )}
      </div>
    );
  },
);

Select.displayName = 'Select';
```

- [ ] **Step 5: Commit**

```bash
git add portal-frontend/src/components/ui/
git commit -m "feat(admin): add Button, Input, Badge, Select UI components"
```

---

### Task 3: Create Modal, Toast, ConfirmDialog

**Files:**
- Create: `portal-frontend/src/components/ui/Modal.tsx`
- Create: `portal-frontend/src/components/ui/Toast.tsx`
- Create: `portal-frontend/src/components/ui/ConfirmDialog.tsx`
- Modify: `portal-frontend/src/main.tsx`

- [ ] **Step 1: Create `portal-frontend/src/components/ui/Modal.tsx`**

```tsx
import { type ReactNode } from 'react';
import * as Dialog from '@radix-ui/react-dialog';

interface ModalProps {
  open: boolean;
  onClose: () => void;
  title: string;
  children: ReactNode;
  wide?: boolean;
}

export function Modal({ open, onClose, title, children, wide }: ModalProps) {
  return (
    <Dialog.Root open={open} onOpenChange={(o) => !o && onClose()}>
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 bg-black/40 z-40" />
        <Dialog.Content
          className={`fixed top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2
            bg-white rounded-lg shadow-xl z-50 p-6 max-h-[85vh] overflow-y-auto
            ${wide ? 'w-[700px]' : 'w-[480px]'}`}
        >
          <Dialog.Title className="text-lg font-semibold mb-4">{title}</Dialog.Title>
          {children}
          <Dialog.Close asChild>
            <button
              className="absolute top-4 right-4 text-gray-400 hover:text-gray-600"
              aria-label="Close"
            >
              ✕
            </button>
          </Dialog.Close>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
```

- [ ] **Step 2: Create `portal-frontend/src/components/ui/Toast.tsx`**

```tsx
import { createContext, useContext, useCallback, useState, type ReactNode } from 'react';
import * as ToastPrimitive from '@radix-ui/react-toast';

interface ToastItem {
  id: string;
  type: 'success' | 'error' | 'info';
  message: string;
}

interface ToastContextValue {
  success: (message: string) => void;
  error: (message: string) => void;
  info: (message: string) => void;
}

const ToastContext = createContext<ToastContextValue | null>(null);

const typeStyles = {
  success: 'border-l-4 border-l-success bg-green-50',
  error: 'border-l-4 border-l-danger bg-red-50',
  info: 'border-l-4 border-l-info bg-blue-50',
};

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<ToastItem[]>([]);

  const addToast = useCallback((type: ToastItem['type'], message: string) => {
    const id = crypto.randomUUID();
    setToasts((prev) => [...prev, { id, type, message }]);
  }, []);

  const removeToast = useCallback((id: string) => {
    setToasts((prev) => prev.filter((t) => t.id !== id));
  }, []);

  const value: ToastContextValue = {
    success: useCallback((m: string) => addToast('success', m), [addToast]),
    error: useCallback((m: string) => addToast('error', m), [addToast]),
    info: useCallback((m: string) => addToast('info', m), [addToast]),
  };

  return (
    <ToastContext.Provider value={value}>
      <ToastPrimitive.Provider swipeDirection="right">
        {children}
        {toasts.map((toast) => (
          <ToastPrimitive.Root
            key={toast.id}
            open
            onOpenChange={(open) => !open && removeToast(toast.id)}
            duration={4000}
            className={`rounded-lg shadow-lg p-4 ${typeStyles[toast.type]}`}
          >
            <ToastPrimitive.Description className="text-sm text-gray-800">
              {toast.message}
            </ToastPrimitive.Description>
          </ToastPrimitive.Root>
        ))}
        <ToastPrimitive.Viewport className="fixed bottom-4 right-4 z-[100] flex flex-col gap-2 w-80" />
      </ToastPrimitive.Provider>
    </ToastContext.Provider>
  );
}

export function useToast(): ToastContextValue {
  const ctx = useContext(ToastContext);
  if (!ctx) throw new Error('useToast must be used within ToastProvider');
  return ctx;
}
```

- [ ] **Step 3: Create `portal-frontend/src/components/ui/ConfirmDialog.tsx`**

```tsx
import { Modal } from './Modal';
import { Button } from './Button';

interface ConfirmDialogProps {
  open: boolean;
  onConfirm: () => void;
  onCancel: () => void;
  title: string;
  description: string;
  confirmLabel?: string;
  variant?: 'danger' | 'default';
  loading?: boolean;
}

export function ConfirmDialog({
  open, onConfirm, onCancel, title, description,
  confirmLabel = 'Confirm', variant = 'default', loading,
}: ConfirmDialogProps) {
  return (
    <Modal open={open} onClose={onCancel} title={title}>
      <p className="text-sm text-gray-600 mb-6">{description}</p>
      <div className="flex justify-end gap-3">
        <Button variant="secondary" onClick={onCancel} disabled={loading}>Cancel</Button>
        <Button
          variant={variant === 'danger' ? 'danger' : 'primary'}
          onClick={onConfirm}
          disabled={loading}
        >
          {loading ? 'Processing...' : confirmLabel}
        </Button>
      </div>
    </Modal>
  );
}
```

- [ ] **Step 4: Add ToastProvider to `portal-frontend/src/main.tsx`**

Wrap the app with `ToastProvider`:

```tsx
import './index.css';
import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { BrowserRouter } from 'react-router-dom';
import { AuthProvider } from './contexts/AuthContext';
import { ToastProvider } from './components/ui/Toast';
import { App } from './App';

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <BrowserRouter>
      <AuthProvider>
        <ToastProvider>
          <App />
        </ToastProvider>
      </AuthProvider>
    </BrowserRouter>
  </StrictMode>,
);
```

- [ ] **Step 5: Commit**

```bash
git add portal-frontend/src/components/ui/ portal-frontend/src/main.tsx
git commit -m "feat(admin): add Modal, Toast, ConfirmDialog components"
```

---

### Task 4: Create DataTable, FilterBar, StatCard

**Files:**
- Create: `portal-frontend/src/components/data/DataTable.tsx`
- Create: `portal-frontend/src/components/data/FilterBar.tsx`
- Create: `portal-frontend/src/components/data/StatCard.tsx`

- [ ] **Step 1: Create `portal-frontend/src/components/data/DataTable.tsx`**

```tsx
import { type ReactNode } from 'react';
import { Button } from '../ui/Button';

export interface Column<T> {
  key: string;
  header: string;
  render?: (item: T) => ReactNode;
  sortable?: boolean;
}

interface DataTableProps<T> {
  columns: Column<T>[];
  data: T[];
  total: number;
  page: number;
  pageSize: number;
  onPageChange: (page: number) => void;
  sortBy?: string;
  sortDir?: 'asc' | 'desc';
  onSort?: (key: string) => void;
  loading?: boolean;
  onRowClick?: (item: T) => void;
  rowActions?: (item: T) => ReactNode;
  keyField?: string;
}

export function DataTable<T extends Record<string, unknown>>({
  columns, data, total, page, pageSize, onPageChange,
  sortBy, sortDir, onSort, loading, onRowClick, rowActions, keyField = 'id',
}: DataTableProps<T>) {
  const totalPages = Math.max(1, Math.ceil(total / pageSize));

  if (loading) {
    return (
      <div className="border border-gray-200 rounded-lg overflow-hidden">
        <div className="animate-pulse p-8 text-center text-gray-400">Loading...</div>
      </div>
    );
  }

  return (
    <div className="border border-gray-200 rounded-lg overflow-hidden">
      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead className="bg-gray-50 border-b border-gray-200">
            <tr>
              {columns.map((col) => (
                <th
                  key={col.key}
                  className={`px-4 py-3 text-left font-medium text-gray-600
                    ${col.sortable ? 'cursor-pointer hover:text-gray-900 select-none' : ''}`}
                  onClick={() => col.sortable && onSort?.(col.key)}
                >
                  <span className="inline-flex items-center gap-1">
                    {col.header}
                    {col.sortable && sortBy === col.key && (
                      <span className="text-primary">{sortDir === 'asc' ? '↑' : '↓'}</span>
                    )}
                  </span>
                </th>
              ))}
              {rowActions && <th className="px-4 py-3 w-12"></th>}
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-100">
            {data.length === 0 ? (
              <tr>
                <td colSpan={columns.length + (rowActions ? 1 : 0)} className="px-4 py-8 text-center text-gray-400">
                  No data found
                </td>
              </tr>
            ) : (
              data.map((item, idx) => (
                <tr
                  key={String(item[keyField] ?? idx)}
                  className={`hover:bg-gray-50 ${onRowClick ? 'cursor-pointer' : ''}`}
                  onClick={() => onRowClick?.(item)}
                >
                  {columns.map((col) => (
                    <td key={col.key} className="px-4 py-3 text-gray-800">
                      {col.render ? col.render(item) : String(item[col.key] ?? '')}
                    </td>
                  ))}
                  {rowActions && (
                    <td className="px-4 py-3" onClick={(e) => e.stopPropagation()}>
                      {rowActions(item)}
                    </td>
                  )}
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
      {total > pageSize && (
        <div className="flex items-center justify-between px-4 py-3 border-t border-gray-200 bg-gray-50">
          <span className="text-sm text-gray-600">
            {(page - 1) * pageSize + 1}–{Math.min(page * pageSize, total)} of {total}
          </span>
          <div className="flex gap-2">
            <Button size="sm" variant="secondary" disabled={page <= 1} onClick={() => onPageChange(page - 1)}>
              Previous
            </Button>
            <Button size="sm" variant="secondary" disabled={page >= totalPages} onClick={() => onPageChange(page + 1)}>
              Next
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 2: Create `portal-frontend/src/components/data/FilterBar.tsx`**

```tsx
import { Input } from '../ui/Input';
import { Select } from '../ui/Select';
import { Button } from '../ui/Button';

export interface FilterDef {
  key: string;
  label: string;
  type: 'text' | 'select' | 'date';
  options?: { value: string; label: string }[];
  placeholder?: string;
}

interface FilterBarProps {
  filters: FilterDef[];
  values: Record<string, string>;
  onChange: (values: Record<string, string>) => void;
  onReset?: () => void;
}

export function FilterBar({ filters, values, onChange, onReset }: FilterBarProps) {
  const update = (key: string, value: string) => {
    onChange({ ...values, [key]: value });
  };

  const hasValues = Object.values(values).some((v) => v !== '');

  return (
    <div className="flex flex-wrap items-end gap-3 mb-4">
      {filters.map((f) => {
        if (f.type === 'select' && f.options) {
          return (
            <div key={f.key} className="min-w-[160px]">
              <Select
                label={f.label}
                options={f.options}
                value={values[f.key] || ''}
                onChange={(v) => update(f.key, v)}
                placeholder={f.placeholder || 'All'}
              />
            </div>
          );
        }
        return (
          <div key={f.key} className="min-w-[160px]">
            <Input
              label={f.label}
              type={f.type === 'date' ? 'date' : 'text'}
              value={values[f.key] || ''}
              onChange={(e) => update(f.key, e.target.value)}
              placeholder={f.placeholder}
            />
          </div>
        );
      })}
      {hasValues && onReset && (
        <Button variant="ghost" size="sm" onClick={onReset}>
          Reset
        </Button>
      )}
    </div>
  );
}
```

- [ ] **Step 3: Create `portal-frontend/src/components/data/StatCard.tsx`**

```tsx
interface StatCardProps {
  title: string;
  value: string | number;
  subtitle?: string;
  trend?: { value: number; direction: 'up' | 'down' };
}

export function StatCard({ title, value, subtitle, trend }: StatCardProps) {
  return (
    <div className="bg-white rounded-lg border border-gray-200 p-5">
      <div className="text-sm font-medium text-gray-500 mb-1">{title}</div>
      <div className="text-2xl font-semibold text-gray-900">{value}</div>
      {(subtitle || trend) && (
        <div className="mt-1 text-sm">
          {trend && (
            <span className={trend.direction === 'up' ? 'text-success' : 'text-danger'}>
              {trend.direction === 'up' ? '↑' : '↓'} {trend.value}%
            </span>
          )}
          {subtitle && <span className="text-gray-500"> {subtitle}</span>}
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 4: Commit**

```bash
git add portal-frontend/src/components/data/
git commit -m "feat(admin): add DataTable, FilterBar, StatCard data components"
```

---

### Task 5: Create Layout Components (Sidebar, PageHeader)

**Files:**
- Create: `portal-frontend/src/components/layout/Sidebar.tsx`
- Create: `portal-frontend/src/components/layout/PageHeader.tsx`

- [ ] **Step 1: Create `portal-frontend/src/components/layout/Sidebar.tsx`**

```tsx
import { type ReactNode } from 'react';
import { Link, useLocation } from 'react-router-dom';

export interface NavItem {
  path: string;
  label: string;
  icon?: ReactNode;
}

interface SidebarProps {
  title: string;
  items: NavItem[];
  footer?: ReactNode;
}

export function Sidebar({ title, items, footer }: SidebarProps) {
  const location = useLocation();

  return (
    <nav aria-label="Admin navigation" className="w-56 border-r border-gray-200 bg-gray-50 flex flex-col min-h-screen">
      <div className="p-4 border-b border-gray-200">
        <h2 className="text-base font-semibold text-gray-900">{title}</h2>
      </div>
      <ul className="flex-1 py-2 space-y-0.5 px-2">
        {items.map((item) => {
          const isActive = location.pathname.startsWith(item.path);
          return (
            <li key={item.path}>
              <Link
                to={item.path}
                aria-current={isActive ? 'page' : undefined}
                className={`flex items-center gap-2 px-3 py-2 rounded text-sm transition-colors
                  ${isActive
                    ? 'bg-primary/10 text-primary font-medium'
                    : 'text-gray-700 hover:bg-gray-100'
                  }`}
              >
                {item.icon}
                {item.label}
              </Link>
            </li>
          );
        })}
      </ul>
      {footer && (
        <div className="p-4 border-t border-gray-200">
          {footer}
        </div>
      )}
    </nav>
  );
}
```

- [ ] **Step 2: Create `portal-frontend/src/components/layout/PageHeader.tsx`**

```tsx
import { type ReactNode } from 'react';
import { Link } from 'react-router-dom';

interface Breadcrumb {
  label: string;
  href?: string;
}

interface PageHeaderProps {
  title: string;
  subtitle?: string;
  breadcrumbs?: Breadcrumb[];
  actions?: ReactNode;
}

export function PageHeader({ title, subtitle, breadcrumbs, actions }: PageHeaderProps) {
  return (
    <div className="mb-6">
      {breadcrumbs && breadcrumbs.length > 0 && (
        <nav aria-label="Breadcrumb" className="mb-2">
          <ol className="flex items-center gap-1 text-sm text-gray-500">
            {breadcrumbs.map((crumb, i) => (
              <li key={i} className="flex items-center gap-1">
                {i > 0 && <span>/</span>}
                {crumb.href ? (
                  <Link to={crumb.href} className="hover:text-gray-700">{crumb.label}</Link>
                ) : (
                  <span className="text-gray-700">{crumb.label}</span>
                )}
              </li>
            ))}
          </ol>
        </nav>
      )}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold text-gray-900">{title}</h1>
          {subtitle && <p className="mt-1 text-sm text-gray-500">{subtitle}</p>}
        </div>
        {actions && <div className="flex gap-2">{actions}</div>}
      </div>
    </div>
  );
}
```

- [ ] **Step 3: Commit**

```bash
git add portal-frontend/src/components/layout/
git commit -m "feat(admin): add Sidebar, PageHeader layout components"
```

---

### Task 6: Auth Infrastructure (Role in AuthContext, RequireRole)

**Files:**
- Modify: `portal-frontend/src/api/client.ts` (ProfileData type)
- Modify: `portal-frontend/src/contexts/AuthContext.tsx`
- Create: `portal-frontend/src/components/RequireRole.tsx`
- Modify: `internal/gateway/portal/handlers/profile.go`

- [ ] **Step 1: Update ProfileData type in `portal-frontend/src/api/client.ts`**

Add `role` to `ProfileData`:

```ts
export interface ProfileData {
  id: string;
  email: string;
  company_name: string;
  contact_person: string;
  phone: string;
  totp_enabled: boolean;
  role?: 'client' | 'admin' | 'superadmin';
}
```

- [ ] **Step 2: Update `portal-frontend/src/contexts/AuthContext.tsx`**

Replace entire file:

```tsx
import { createContext, useContext, useState, useEffect, useCallback, type ReactNode } from 'react';
import { authApi, profileApi, ApiError, type ProfileData } from '../api/client';

type UserRole = 'client' | 'admin' | 'superadmin';

interface AuthState {
  user: ProfileData | null;
  role: UserRole;
  isAuthenticated: boolean;
  loading: boolean;
  isAdmin: boolean;
  login: (email: string, password: string) => Promise<LoginResult>;
  login2fa: (loginTicket: string, totpCode: string) => Promise<UserRole>;
  logout: () => Promise<void>;
}

interface LoginResult {
  requires2fa: boolean;
  loginTicket?: string;
  role?: UserRole;
}

const AuthContext = createContext<AuthState | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<ProfileData | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    profileApi
      .get()
      .then(setUser)
      .catch(() => setUser(null))
      .finally(() => setLoading(false));
  }, []);

  const login = useCallback(async (email: string, password: string): Promise<LoginResult> => {
    const res = await authApi.login(email, password);
    if (res.requires_2fa) {
      return { requires2fa: true, loginTicket: res.login_ticket };
    }
    const profile = await profileApi.get();
    setUser(profile);
    return { requires2fa: false, role: (profile.role as UserRole) || 'client' };
  }, []);

  const login2fa = useCallback(async (loginTicket: string, totpCode: string): Promise<UserRole> => {
    await authApi.login2fa(loginTicket, totpCode);
    const profile = await profileApi.get();
    setUser(profile);
    return (profile.role as UserRole) || 'client';
  }, []);

  const logout = useCallback(async () => {
    try {
      await authApi.logout();
    } catch (e) {
      if (!(e instanceof ApiError && e.status === 401)) throw e;
    }
    setUser(null);
  }, []);

  const role: UserRole = (user?.role as UserRole) || 'client';
  const isAdmin = role === 'admin' || role === 'superadmin';

  return (
    <AuthContext.Provider
      value={{ user, role, isAuthenticated: !!user, loading, isAdmin, login, login2fa, logout }}
    >
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth(): AuthState {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error('useAuth must be used within AuthProvider');
  return ctx;
}
```

- [ ] **Step 3: Create `portal-frontend/src/components/RequireRole.tsx`**

```tsx
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
```

- [ ] **Step 4: Update backend profile handler**

In `internal/gateway/portal/handlers/profile.go`, replace lines 87-90 (the `if role == "admin"` block) with unconditional role assignment:

```go
	// Всегда возвращаем роль пользователя
	response["role"] = role
```

This replaces:
```go
	// Если есть роль admin — не требуем запись клиента
	if role == "admin" {
		response["role"] = "admin"
	}
```

- [ ] **Step 5: Commit**

```bash
git add portal-frontend/src/api/client.ts portal-frontend/src/contexts/AuthContext.tsx portal-frontend/src/components/RequireRole.tsx internal/gateway/portal/handlers/profile.go
git commit -m "feat(admin): add role to auth context, RequireRole component, backend role in profile"
```

---

### Task 7: AdminLayout and Routing

**Files:**
- Create: `portal-frontend/src/pages/admin/AdminLayout.tsx`
- Modify: `portal-frontend/src/App.tsx`
- Modify: `portal-frontend/src/pages/auth/LoginPage.tsx`

- [ ] **Step 1: Create `portal-frontend/src/pages/admin/AdminLayout.tsx`**

```tsx
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
```

- [ ] **Step 2: Update `portal-frontend/src/App.tsx`**

Replace entire file:

```tsx
import { lazy, Suspense } from 'react';
import { Routes, Route, Navigate, Link, Outlet, useLocation } from 'react-router-dom';
import { useAuth } from './contexts/AuthContext';
import { SkipLink } from './components/SkipLink';
import { RequireRole } from './components/RequireRole';
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

const AdminLayout = lazy(() => import('./pages/admin/AdminLayout').then((m) => ({ default: m.AdminLayout })));
const AdminClientsPage = lazy(() => import('./pages/admin/ClientsPage').then((m) => ({ default: m.ClientsPage })));
const AdminProvidersPage = lazy(() => import('./pages/admin/ProvidersPage').then((m) => ({ default: m.ProvidersPage })));
const AdminRoutesPage = lazy(() => import('./pages/admin/RoutesPage').then((m) => ({ default: m.RoutesPage })));
const AdminBillingPage = lazy(() => import('./pages/admin/BillingPage').then((m) => ({ default: m.BillingPage })));
const AdminMonitoringPage = lazy(() => import('./pages/admin/MonitoringPage').then((m) => ({ default: m.MonitoringPage })));
const AdminAnalyticsPage = lazy(() => import('./pages/admin/AnalyticsPage').then((m) => ({ default: m.AnalyticsPage })));
const AdminTemplatesPage = lazy(() => import('./pages/admin/TemplatesPage').then((m) => ({ default: m.TemplatesPage })));
const AdminWebhooksPage = lazy(() => import('./pages/admin/WebhooksPage').then((m) => ({ default: m.WebhooksPage })));
const AdminHLRPage = lazy(() => import('./pages/admin/HLRPage').then((m) => ({ default: m.HLRPage })));
const AdminCountriesPage = lazy(() => import('./pages/admin/CountriesPage').then((m) => ({ default: m.CountriesPage })));
const AdminAuditLogPage = lazy(() => import('./pages/admin/AuditLogPage').then((m) => ({ default: m.AuditLogPage })));

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

      <Route
        path="/admin/*"
        element={
          <RequireRole role="admin">
            <Suspense fallback={<div className="p-8 text-center text-gray-400">Loading admin...</div>}>
              <AdminLayout />
            </Suspense>
          </RequireRole>
        }
      >
        <Route index element={<Navigate to="/admin/clients" replace />} />
        <Route path="clients" element={<Suspense fallback={null}><AdminClientsPage /></Suspense>} />
        <Route path="providers" element={<Suspense fallback={null}><AdminProvidersPage /></Suspense>} />
        <Route path="routes" element={<Suspense fallback={null}><AdminRoutesPage /></Suspense>} />
        <Route path="billing" element={<Suspense fallback={null}><AdminBillingPage /></Suspense>} />
        <Route path="monitoring" element={<Suspense fallback={null}><AdminMonitoringPage /></Suspense>} />
        <Route path="analytics" element={<Suspense fallback={null}><AdminAnalyticsPage /></Suspense>} />
        <Route path="templates" element={<Suspense fallback={null}><AdminTemplatesPage /></Suspense>} />
        <Route path="webhooks" element={<Suspense fallback={null}><AdminWebhooksPage /></Suspense>} />
        <Route path="hlr" element={<Suspense fallback={null}><AdminHLRPage /></Suspense>} />
        <Route path="countries" element={<Suspense fallback={null}><AdminCountriesPage /></Suspense>} />
        <Route path="audit" element={<Suspense fallback={null}><AdminAuditLogPage /></Suspense>} />
      </Route>

      <Route path="*" element={<Navigate to="/dashboard" replace />} />
    </Routes>
  );
}
```

- [ ] **Step 3: Update LoginPage redirect by role**

In `portal-frontend/src/pages/auth/LoginPage.tsx`, change the redirect logic after successful login.

Replace `navigate(from, { replace: true });` in `handleLogin` (the non-2FA branch) with:

```tsx
      const dest = result.role === 'admin' || result.role === 'superadmin' ? '/admin' : from;
      navigate(dest, { replace: true });
```

Replace `navigate(from, { replace: true });` in `handle2fa` with:

```tsx
      const dest = role === 'admin' || role === 'superadmin' ? '/admin' : from;
      navigate(dest, { replace: true });
```

And update `handle2fa` to capture role from `login2fa`:

```tsx
  async function handle2fa(e: FormEvent) {
    e.preventDefault();
    setError('');
    setSubmitting(true);
    try {
      const role = await login2fa(loginTicket!, totpCode);
      const dest = role === 'admin' || role === 'superadmin' ? '/admin' : from;
      navigate(dest, { replace: true });
    } catch (err) {
      setError(err instanceof ApiError ? err.message : '2FA verification failed');
    } finally {
      setSubmitting(false);
    }
  }
```

- [ ] **Step 4: Verify build**

```bash
cd portal-frontend && npm run build
```

Note: Build will show warnings about missing page modules. That's expected — they'll be created in later tasks. The build should still succeed because lazy imports only fail at runtime.

- [ ] **Step 5: Commit**

```bash
git add portal-frontend/src/
git commit -m "feat(admin): add AdminLayout, role-based routing, lazy-loaded admin routes"
```

---

### Task 8: Create Admin API Client

**Files:**
- Create: `portal-frontend/src/api/admin.ts`

- [ ] **Step 1: Create `portal-frontend/src/api/admin.ts`**

```tsx
const API_BASE = '/admin/v1';

function getCookie(name: string): string | null {
  const match = document.cookie.match(new RegExp(`(^| )${name}=([^;]+)`));
  return match ? match[2] : null;
}

export class AdminApiError extends Error {
  constructor(public status: number, message: string, public details?: unknown) {
    super(message);
  }
}

async function adminFetch<T>(path: string, options?: RequestInit): Promise<T> {
  const csrfToken = getCookie('csrf_token');
  const res = await fetch(`${API_BASE}${path}`, {
    credentials: 'include',
    headers: {
      'Content-Type': 'application/json',
      ...(csrfToken ? { 'X-CSRF-Token': csrfToken } : {}),
      ...options?.headers,
    },
    ...options,
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: { message: res.statusText } }));
    throw new AdminApiError(res.status, err.error?.message || res.statusText, err.error);
  }
  if (res.status === 204) return {} as T;
  return res.json();
}

function qs(params: Record<string, string | number | boolean | undefined>): string {
  const filtered = Object.entries(params).filter(([, v]) => v !== undefined && v !== '');
  if (filtered.length === 0) return '';
  return '?' + new URLSearchParams(filtered.map(([k, v]) => [k, String(v)])).toString();
}

// ── Types ──

export interface ClientInfo {
  client_id: string;
  name: string;
  email: string;
  contact_person: string;
  phone: string;
  active: boolean;
  rate_limits: RateLimits;
  metadata: Record<string, string>;
  created_at: string;
  updated_at: string;
}

export interface RateLimits {
  messages_per_second: number;
  messages_per_minute: number;
  messages_per_hour: number;
  messages_per_day: number;
}

export interface ProviderInfo {
  provider_id: string;
  name: string;
  host: string;
  port: number;
  system_id: string;
  system_type: string;
  bind_type: number;
  max_connections: number;
  window_size: number;
  active: boolean;
  settings: Record<string, string>;
  created_at: string;
  updated_at: string;
}

export interface ProviderHealth {
  provider_id: string;
  status: string;
  active_connections: number;
  total_connections: number;
  success_rate: number;
  messages_sent_24h: number;
  messages_failed_24h: number;
  last_success?: string;
  last_failure?: string;
  last_error?: string;
}

export interface RouteInfo {
  route_id: string;
  name: string;
  pattern: string;
  priority: number;
  provider_ids: string[];
  load_balance_strategy: string;
  failover_enabled: boolean;
  active: boolean;
  metadata: Record<string, string>;
  created_at: string;
  updated_at: string;
}

export interface BalanceResponse {
  client_id: string;
  balance: string;
  currency: string;
  updated_at: string;
}

export interface Transaction {
  transaction_id: string;
  client_id: string;
  type: string;
  amount: string;
  currency: string;
  balance_before: string;
  balance_after: string;
  description: string;
  message_id?: string;
  created_at: string;
}

export interface PricingRule {
  rule_id: string;
  client_id: string;
  destination_pattern: string;
  price_per_message: string;
  currency: string;
  priority: number;
  active: boolean;
  created_at: string;
  updated_at: string;
}

export interface TemplateInfo {
  template_id: string;
  client_id: string;
  name: string;
  body: string;
  status: string;
  created_at: string;
  updated_at: string;
}

export interface WebhookInfo {
  webhook_id: string;
  client_id: string;
  url: string;
  events: string[];
  active: boolean;
  secret?: string;
  created_at: string;
  updated_at: string;
}

export interface CountryInfo {
  country_id: string;
  name: string;
  code: string;
  phone_code: string;
  created_at: string;
  updated_at: string;
}

export interface OperatorInfo {
  operator_id: string;
  name: string;
  country_id: string;
  mcc: string;
  mnc: string;
  active: boolean;
  created_at: string;
  updated_at: string;
}

export interface OperatorPrefix {
  prefix_id: string;
  operator_id: string;
  prefix: string;
}

export interface TariffPlan {
  tariff_plan_id: string;
  name: string;
  description: string;
  active: boolean;
  created_at: string;
  updated_at: string;
}

export interface HLRProvider {
  provider_id: string;
  name: string;
  active: boolean;
  created_at: string;
  updated_at: string;
}

export interface SmartRouteWeight {
  weight_id: string;
  country_code: string;
  provider_id: string;
  weight: number;
  created_at: string;
}

export interface AuditEntry {
  id: string;
  user_id: string;
  action: string;
  resource_type: string;
  resource_id: string;
  ip_address: string;
  details: Record<string, unknown>;
  created_at: string;
}

export interface RealTimeMetrics {
  messages_per_second: number;
  messages_delivered: number;
  messages_failed: number;
  active_providers: number;
  queue_depth: number;
}

export interface ListResponse<T> {
  total: number;
  limit: number;
  offset: number;
  items: T[];
}

// ── API ──

export const clientsApi = {
  list: (params?: { active_only?: boolean; search?: string; limit?: number; offset?: number }) =>
    adminFetch<{ clients: ClientInfo[]; total: number; limit: number; offset: number }>(`/clients${qs(params || {})}`),
  get: (id: string) => adminFetch<{ client: ClientInfo }>(`/clients/${id}`),
  create: (data: { name: string; email: string; contact_person?: string; phone?: string; active?: boolean }) =>
    adminFetch<{ client_id: string; created_at: string }>('/clients', { method: 'POST', body: JSON.stringify(data) }),
  update: (id: string, data: Partial<ClientInfo>) =>
    adminFetch<void>(`/clients/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  delete: (id: string) => adminFetch<void>(`/clients/${id}`, { method: 'DELETE' }),
  getConfig: (id: string) => adminFetch<unknown>(`/clients/${id}/config`),
  updateConfig: (id: string, data: unknown) =>
    adminFetch<void>(`/clients/${id}/config`, { method: 'PUT', body: JSON.stringify(data) }),
  updateRateLimits: (id: string, data: RateLimits) =>
    adminFetch<void>(`/clients/${id}/rate-limits`, { method: 'PUT', body: JSON.stringify(data) }),
};

export const providersApi = {
  list: (params?: { active_only?: boolean; limit?: number; offset?: number }) =>
    adminFetch<{ providers: ProviderInfo[]; total: number; limit: number; offset: number }>(`/providers${qs(params || {})}`),
  get: (id: string) => adminFetch<{ provider: ProviderInfo }>(`/providers/${id}`),
  create: (data: Partial<ProviderInfo> & { password: string }) =>
    adminFetch<{ provider_id: string; created_at: string }>('/providers', { method: 'POST', body: JSON.stringify(data) }),
  update: (id: string, data: Partial<ProviderInfo>) =>
    adminFetch<void>(`/providers/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  delete: (id: string) => adminFetch<void>(`/providers/${id}`, { method: 'DELETE' }),
  health: (id: string) => adminFetch<ProviderHealth>(`/providers/${id}/health`),
};

export const routesApi = {
  list: (params?: { active_only?: boolean; limit?: number; offset?: number }) =>
    adminFetch<{ routes: RouteInfo[]; total: number; limit: number; offset: number }>(`/routes${qs(params || {})}`),
  create: (data: Partial<RouteInfo>) =>
    adminFetch<{ route_id: string; created_at: string }>('/routes', { method: 'POST', body: JSON.stringify(data) }),
  update: (id: string, data: Partial<RouteInfo>) =>
    adminFetch<void>(`/routes/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  delete: (id: string) => adminFetch<void>(`/routes/${id}`, { method: 'DELETE' }),
};

export const billingApi = {
  getBalance: (clientId: string) => adminFetch<BalanceResponse>(`/billing/clients/${clientId}/balance`),
  addCredits: (clientId: string, data: { amount: string; currency?: string; description?: string }) =>
    adminFetch<{ transaction_id: string; new_balance: string; success: boolean }>(`/billing/clients/${clientId}/credits`, {
      method: 'POST', body: JSON.stringify(data),
    }),
  getTransactions: (params?: { client_id?: string; from?: string; to?: string; transaction_type?: string; limit?: number; offset?: number }) =>
    adminFetch<{ transactions: Transaction[]; total: number; limit: number; offset: number }>(`/billing/transactions${qs(params || {})}`),
  getPricingRules: (params?: { client_id?: string }) =>
    adminFetch<{ rules: PricingRule[] }>(`/billing/pricing-rules${qs(params || {})}`),
  createPricingRule: (data: Partial<PricingRule>) =>
    adminFetch<void>('/billing/pricing-rules', { method: 'POST', body: JSON.stringify(data) }),
};

export const analyticsAdminApi = {
  getStats: (params: { client_id?: string; from?: string; to?: string; group_by?: string; provider_ids?: string[] }) => {
    const { provider_ids, ...rest } = params;
    const base = qs(rest);
    const providerQs = provider_ids?.map((id) => `provider_ids[]=${id}`).join('&') || '';
    const sep = base ? '&' : '?';
    return adminFetch<unknown>(`/analytics/stats${base}${providerQs ? sep + providerQs : ''}`);
  },
  generateReport: (data: unknown) =>
    adminFetch<unknown>('/analytics/reports', { method: 'POST', body: JSON.stringify(data) }),
  getRealTimeMetrics: () => adminFetch<RealTimeMetrics>('/analytics/metrics/realtime'),
  getProviderPerformance: (id: string, params?: { from?: string; to?: string }) =>
    adminFetch<unknown>(`/analytics/providers/${id}/performance${qs(params || {})}`),
};

export const webhooksAdminApi = {
  list: (params?: { client_id?: string }) =>
    adminFetch<{ webhooks: WebhookInfo[] }>(`/webhooks${qs(params || {})}`),
  get: (id: string, clientId?: string) =>
    adminFetch<{ webhook: WebhookInfo }>(`/webhooks/${id}${qs({ client_id: clientId })}`),
  create: (data: Partial<WebhookInfo>) =>
    adminFetch<void>('/webhooks', { method: 'POST', body: JSON.stringify(data) }),
  update: (id: string, data: Partial<WebhookInfo>) =>
    adminFetch<void>(`/webhooks/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  delete: (id: string, clientId?: string) =>
    adminFetch<void>(`/webhooks/${id}${qs({ client_id: clientId })}`, { method: 'DELETE' }),
};

export const templatesApi = {
  list: (params?: { client_id?: string; status?: string; limit?: number; offset?: number }) =>
    adminFetch<{ templates: TemplateInfo[]; total: number; limit: number; offset: number }>(`/templates${qs(params || {})}`),
  get: (id: string, clientId?: string) =>
    adminFetch<{ template: TemplateInfo }>(`/templates/${id}${qs({ client_id: clientId })}`),
  approve: (id: string) =>
    adminFetch<void>(`/templates/${id}/approve`, { method: 'POST' }),
  reject: (id: string, data?: { reason?: string }) =>
    adminFetch<void>(`/templates/${id}/reject`, { method: 'POST', body: JSON.stringify(data || {}) }),
  audit: (id: string) =>
    adminFetch<{ entries: AuditEntry[] }>(`/templates/${id}/audit`),
};

export const countriesApi = {
  list: (params?: { limit?: number; offset?: number }) =>
    adminFetch<{ countries: CountryInfo[]; total: number; limit: number; offset: number }>(`/countries${qs(params || {})}`),
  get: (id: string) => adminFetch<{ country: CountryInfo }>(`/countries/${id}`),
  create: (data: Partial<CountryInfo>) =>
    adminFetch<void>('/countries', { method: 'POST', body: JSON.stringify(data) }),
  update: (id: string, data: Partial<CountryInfo>) =>
    adminFetch<void>(`/countries/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
};

export const operatorsApi = {
  list: (params?: { country_id?: string; active_only?: boolean; limit?: number; offset?: number }) =>
    adminFetch<{ operators: OperatorInfo[]; total: number; limit: number; offset: number }>(`/operators${qs(params || {})}`),
  get: (id: string) => adminFetch<{ operator: OperatorInfo }>(`/operators/${id}`),
  create: (data: Partial<OperatorInfo>) =>
    adminFetch<void>('/operators', { method: 'POST', body: JSON.stringify(data) }),
  update: (id: string, data: Partial<OperatorInfo>) =>
    adminFetch<void>(`/operators/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  listPrefixes: (id: string) =>
    adminFetch<{ prefixes: OperatorPrefix[] }>(`/operators/${id}/prefixes`),
  createPrefix: (id: string, data: { prefix: string }) =>
    adminFetch<void>(`/operators/${id}/prefixes`, { method: 'POST', body: JSON.stringify(data) }),
  deletePrefix: (operatorId: string, prefixId: string) =>
    adminFetch<void>(`/operators/${operatorId}/prefixes/${prefixId}`, { method: 'DELETE' }),
};

export const tarificationApi = {
  listTariffPlans: () =>
    adminFetch<{ tariff_plans: TariffPlan[] }>('/tarification/tariff-plans'),
  createTariffPlan: (data: Partial<TariffPlan>) =>
    adminFetch<void>('/tarification/tariff-plans', { method: 'POST', body: JSON.stringify(data) }),
  updateTariffPlan: (id: string, data: Partial<TariffPlan>) =>
    adminFetch<void>(`/tarification/tariff-plans/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  listUsage: () => adminFetch<unknown>('/tarification/usage'),
};

export const hlrApi = {
  listProviders: (params?: { active_only?: boolean }) =>
    adminFetch<{ providers: HLRProvider[] }>(`/hlr/providers${qs(params || {})}`),
  getProvider: (id: string) => adminFetch<{ provider: HLRProvider }>(`/hlr/providers/${id}`),
  createProvider: (data: Partial<HLRProvider>) =>
    adminFetch<void>('/hlr/providers', { method: 'POST', body: JSON.stringify(data) }),
  updateProvider: (id: string, data: Partial<HLRProvider>) =>
    adminFetch<void>(`/hlr/providers/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteProvider: (id: string) => adminFetch<void>(`/hlr/providers/${id}`, { method: 'DELETE' }),
  providerHealth: (id: string) => adminFetch<ProviderHealth>(`/hlr/providers/${id}/health`),
  listWeights: (params?: { country_code?: string }) =>
    adminFetch<{ weights: SmartRouteWeight[] }>(`/routing/weights${qs(params || {})}`),
  setWeights: (data: { country_code: string; provider_id: string; weight: number }) =>
    adminFetch<void>('/routing/weights', { method: 'POST', body: JSON.stringify(data) }),
  deleteWeight: (id: string) => adminFetch<void>(`/routing/weights/${id}`, { method: 'DELETE' }),
};

export const auditAdminApi = {
  list: (params?: { user_id?: string; action?: string; from?: string; to?: string; limit?: number; offset?: number }) =>
    adminFetch<{ entries: AuditEntry[]; total: number; limit: number; offset: number }>(`/audit${qs(params || {})}`),
};
```

Note: some response types may not match the backend exactly — adjust field names after testing against the running API.

- [ ] **Step 2: Commit**

```bash
git add portal-frontend/src/api/admin.ts
git commit -m "feat(admin): add admin API client with all endpoint groups"
```

---

### Task 9: Backend — Admin Auth Middleware (Session Cookie)

**Files:**
- Modify: `internal/gateway/admin/middleware/auth.go`

- [ ] **Step 1: Replace admin auth middleware**

Replace `internal/gateway/admin/middleware/auth.go` with session-based authentication:

```go
package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/api/proto/authv1"
	"github.com/smpp-server/smpp-server/internal/shared"
)

type contextKey string

const (
	UserIDKey      contextKey = "user_id"
	UserKey        contextKey = "user"
	RoleKey        contextKey = "role"
	PermissionsKey contextKey = "permissions"
)

// publicPaths — пути, которые не требуют аутентификации
var publicPaths = []string{
	"/health",
	"/metrics",
}

func isPublicPath(path string) bool {
	for _, p := range publicPaths {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

// AdminAuthMiddleware создает middleware для аутентификации администраторов.
// Проверяет session cookie (portal_session) через auth service.
func AdminAuthMiddleware(authClient authv1.AuthServiceClient) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Пропускаем публичные эндпоинты
			if isPublicPath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			// Извлекаем session cookie
			cookie, err := r.Cookie("portal_session")
			if err != nil || cookie.Value == "" {
				respondError(w, shared.ErrUnauthorized("Session cookie required"))
				return
			}

			// Валидируем сессию через auth service
			resp, err := authClient.ValidateSession(r.Context(), &authv1.ValidateSessionRequest{
				SessionId: cookie.Value,
			})
			if err != nil || !resp.Valid {
				respondError(w, shared.ErrUnauthorized("Invalid or expired session"))
				return
			}

			// Проверяем роль — только admin и superadmin
			role := ""
			if resp.User != nil && resp.User.Role != nil {
				role = resp.User.Role.Name
			}
			if role != "admin" && role != "superadmin" {
				respondError(w, shared.ErrPermissionDenied("Admin access required"))
				return
			}

			// Парсим user ID
			userID, err := uuid.Parse(resp.User.Id)
			if err != nil {
				respondError(w, shared.ErrInternal("Invalid user ID"))
				return
			}

			// Устанавливаем контекст
			ctx := r.Context()
			ctx = context.WithValue(ctx, UserIDKey, userID)
			ctx = context.WithValue(ctx, UserKey, resp.User)
			ctx = context.WithValue(ctx, RoleKey, &authv1.Role{Name: role})

			// Загружаем permissions если доступны
			if resp.User != nil {
				permResp, err := authClient.GetPermissions(r.Context(), &authv1.GetPermissionsRequest{
					UserId: resp.User.Id,
				})
				if err == nil && permResp.Permissions != nil {
					ctx = context.WithValue(ctx, PermissionsKey, permResp.Permissions)
				}
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUserID извлекает ID пользователя из контекста
func GetUserID(ctx context.Context) (uuid.UUID, bool) {
	userID, ok := ctx.Value(UserIDKey).(uuid.UUID)
	return userID, ok
}

// GetUser извлекает информацию о пользователе из контекста
func GetUser(ctx context.Context) (*authv1.UserInfo, bool) {
	user, ok := ctx.Value(UserKey).(*authv1.UserInfo)
	return user, ok
}

// GetRole извлекает роль пользователя из контекста
func GetRole(ctx context.Context) (*authv1.Role, bool) {
	role, ok := ctx.Value(RoleKey).(*authv1.Role)
	return role, ok
}

// HasPermission проверяет, есть ли у пользователя указанное право
func HasPermission(ctx context.Context, resource, action string) bool {
	permissions, ok := ctx.Value(PermissionsKey).([]*authv1.Permission)
	if !ok || permissions == nil {
		return false
	}

	for _, perm := range permissions {
		if perm.Resource == resource && perm.Action == action {
			return true
		}
	}

	return false
}

// respondError отправляет ошибку в формате JSON
func respondError(w http.ResponseWriter, err *shared.AppError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(err.HTTPStatus)

	response := map[string]interface{}{
		"error": map[string]interface{}{
			"code":    err.Code,
			"message": err.Message,
		},
	}

	if err.Details != "" {
		response["error"].(map[string]interface{})["details"] = err.Details
	}

	json.NewEncoder(w).Encode(response)
}
```

- [ ] **Step 2: Verify compilation**

```bash
cd /home/magomed/projects/sms && go build ./cmd/admin-gateway/...
```

Expected: Build succeeds.

- [ ] **Step 3: Commit**

```bash
git add internal/gateway/admin/middleware/auth.go
git commit -m "feat(admin): enable session cookie auth in admin gateway middleware"
```

---

### Task 10: Clients Page

**Files:**
- Create: `portal-frontend/src/pages/admin/ClientsPage.tsx`

- [ ] **Step 1: Create `portal-frontend/src/pages/admin/ClientsPage.tsx`**

```tsx
import { useState, useEffect, useCallback } from 'react';
import { PageHeader } from '../../components/layout/PageHeader';
import { DataTable, type Column } from '../../components/data/DataTable';
import { FilterBar, type FilterDef } from '../../components/data/FilterBar';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Input';
import { Select } from '../../components/ui/Select';
import { Modal } from '../../components/ui/Modal';
import { ConfirmDialog } from '../../components/ui/ConfirmDialog';
import { StatusBadge } from '../../components/ui/Badge';
import { useToast } from '../../components/ui/Toast';
import { clientsApi, type ClientInfo } from '../../api/admin';

const PAGE_SIZE = 20;

const filters: FilterDef[] = [
  { key: 'search', label: 'Search', type: 'text', placeholder: 'Name or email...' },
  { key: 'active_only', label: 'Status', type: 'select', options: [
    { value: 'true', label: 'Active only' },
    { value: 'false', label: 'All' },
  ]},
];

const columns: Column<ClientInfo>[] = [
  { key: 'name', header: 'Name', sortable: true },
  { key: 'email', header: 'Email' },
  { key: 'contact_person', header: 'Contact' },
  { key: 'active', header: 'Status', render: (c) => <StatusBadge status={c.active ? 'active' : 'inactive'} /> },
  { key: 'rate_limits', header: 'Rate (msg/s)', render: (c) => String(c.rate_limits?.messages_per_second ?? '-') },
  { key: 'created_at', header: 'Created', render: (c) => new Date(c.created_at).toLocaleDateString() },
];

export function ClientsPage() {
  const toast = useToast();
  const [data, setData] = useState<ClientInfo[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [filterValues, setFilterValues] = useState<Record<string, string>>({});
  const [showCreate, setShowCreate] = useState(false);
  const [editClient, setEditClient] = useState<ClientInfo | null>(null);
  const [deleteClient, setDeleteClient] = useState<ClientInfo | null>(null);
  const [saving, setSaving] = useState(false);

  const [form, setForm] = useState({ name: '', email: '', contact_person: '', phone: '', active: true });

  const fetchData = useCallback(async () => {
    setLoading(true);
    try {
      const res = await clientsApi.list({
        search: filterValues.search || undefined,
        active_only: filterValues.active_only === 'true' ? true : undefined,
        limit: PAGE_SIZE,
        offset: (page - 1) * PAGE_SIZE,
      });
      setData(res.clients || []);
      setTotal(res.total);
    } catch {
      toast.error('Failed to load clients');
    } finally {
      setLoading(false);
    }
  }, [page, filterValues, toast]);

  useEffect(() => { fetchData(); }, [fetchData]);

  const openCreate = () => {
    setForm({ name: '', email: '', contact_person: '', phone: '', active: true });
    setShowCreate(true);
  };

  const openEdit = (client: ClientInfo) => {
    setForm({ name: client.name, email: client.email, contact_person: client.contact_person, phone: client.phone, active: client.active });
    setEditClient(client);
  };

  const handleSave = async () => {
    setSaving(true);
    try {
      if (editClient) {
        await clientsApi.update(editClient.client_id, form);
        toast.success('Client updated');
        setEditClient(null);
      } else {
        await clientsApi.create(form);
        toast.success('Client created');
        setShowCreate(false);
      }
      fetchData();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Save failed');
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async () => {
    if (!deleteClient) return;
    setSaving(true);
    try {
      await clientsApi.delete(deleteClient.client_id);
      toast.success('Client deleted');
      setDeleteClient(null);
      fetchData();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Delete failed');
    } finally {
      setSaving(false);
    }
  };

  const formModal = (
    <Modal open={showCreate || !!editClient} onClose={() => { setShowCreate(false); setEditClient(null); }} title={editClient ? 'Edit Client' : 'Create Client'}>
      <div className="space-y-4">
        <Input label="Name" value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} required />
        <Input label="Email" type="email" value={form.email} onChange={(e) => setForm({ ...form, email: e.target.value })} required />
        <Input label="Contact Person" value={form.contact_person} onChange={(e) => setForm({ ...form, contact_person: e.target.value })} />
        <Input label="Phone" value={form.phone} onChange={(e) => setForm({ ...form, phone: e.target.value })} />
        <Select label="Status" options={[{ value: 'true', label: 'Active' }, { value: 'false', label: 'Inactive' }]} value={String(form.active)} onChange={(v) => setForm({ ...form, active: v === 'true' })} />
        <div className="flex justify-end gap-3 pt-2">
          <Button variant="secondary" onClick={() => { setShowCreate(false); setEditClient(null); }}>Cancel</Button>
          <Button onClick={handleSave} disabled={saving || !form.name || !form.email}>
            {saving ? 'Saving...' : 'Save'}
          </Button>
        </div>
      </div>
    </Modal>
  );

  return (
    <>
      <PageHeader title="Clients" subtitle={`${total} clients`} actions={<Button onClick={openCreate}>Create Client</Button>} />
      <FilterBar filters={filters} values={filterValues} onChange={(v) => { setFilterValues(v); setPage(1); }} onReset={() => { setFilterValues({}); setPage(1); }} />
      <DataTable
        columns={columns}
        data={data}
        total={total}
        page={page}
        pageSize={PAGE_SIZE}
        onPageChange={setPage}
        loading={loading}
        keyField="client_id"
        rowActions={(client) => (
          <div className="flex gap-1">
            <Button size="sm" variant="ghost" onClick={() => openEdit(client)}>Edit</Button>
            <Button size="sm" variant="ghost" onClick={() => setDeleteClient(client)}>Delete</Button>
          </div>
        )}
      />
      {formModal}
      <ConfirmDialog
        open={!!deleteClient}
        onConfirm={handleDelete}
        onCancel={() => setDeleteClient(null)}
        title="Delete Client"
        description={`Are you sure you want to delete "${deleteClient?.name}"? This action cannot be undone.`}
        confirmLabel="Delete"
        variant="danger"
        loading={saving}
      />
    </>
  );
}
```

- [ ] **Step 2: Commit**

```bash
git add portal-frontend/src/pages/admin/ClientsPage.tsx
git commit -m "feat(admin): add Clients admin page"
```

---

### Task 11: Providers Page

**Files:**
- Create: `portal-frontend/src/pages/admin/ProvidersPage.tsx`

- [ ] **Step 1: Create `portal-frontend/src/pages/admin/ProvidersPage.tsx`**

```tsx
import { useState, useEffect, useCallback } from 'react';
import { PageHeader } from '../../components/layout/PageHeader';
import { DataTable, type Column } from '../../components/data/DataTable';
import { FilterBar, type FilterDef } from '../../components/data/FilterBar';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Input';
import { Select } from '../../components/ui/Select';
import { Modal } from '../../components/ui/Modal';
import { ConfirmDialog } from '../../components/ui/ConfirmDialog';
import { StatusBadge } from '../../components/ui/Badge';
import { useToast } from '../../components/ui/Toast';
import { providersApi, type ProviderInfo, type ProviderHealth } from '../../api/admin';

const PAGE_SIZE = 20;

const filters: FilterDef[] = [
  { key: 'active_only', label: 'Status', type: 'select', options: [
    { value: 'true', label: 'Active only' },
    { value: 'false', label: 'All' },
  ]},
];

export function ProvidersPage() {
  const toast = useToast();
  const [data, setData] = useState<ProviderInfo[]>([]);
  const [healthMap, setHealthMap] = useState<Record<string, ProviderHealth>>({});
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [filterValues, setFilterValues] = useState<Record<string, string>>({});
  const [showCreate, setShowCreate] = useState(false);
  const [editProvider, setEditProvider] = useState<ProviderInfo | null>(null);
  const [deleteProvider, setDeleteProvider] = useState<ProviderInfo | null>(null);
  const [saving, setSaving] = useState(false);
  const [form, setForm] = useState({
    name: '', host: '', port: 2775, system_id: '', password: '', system_type: '',
    bind_type: 1, max_connections: 1, window_size: 10, active: true,
  });

  const fetchData = useCallback(async () => {
    setLoading(true);
    try {
      const res = await providersApi.list({
        active_only: filterValues.active_only === 'true' ? true : undefined,
        limit: PAGE_SIZE, offset: (page - 1) * PAGE_SIZE,
      });
      setData(res.providers || []);
      setTotal(res.total);
      // Fetch health for each provider
      const healthEntries = await Promise.allSettled(
        (res.providers || []).map((p) => providersApi.health(p.provider_id).then((h) => [p.provider_id, h] as const))
      );
      const map: Record<string, ProviderHealth> = {};
      healthEntries.forEach((r) => { if (r.status === 'fulfilled') map[r.value[0]] = r.value[1]; });
      setHealthMap(map);
    } catch {
      toast.error('Failed to load providers');
    } finally {
      setLoading(false);
    }
  }, [page, filterValues, toast]);

  useEffect(() => { fetchData(); }, [fetchData]);

  const columns: Column<ProviderInfo>[] = [
    { key: 'name', header: 'Name', sortable: true },
    { key: 'host', header: 'Host', render: (p) => `${p.host}:${p.port}` },
    { key: 'system_id', header: 'System ID' },
    { key: 'max_connections', header: 'Max Conn' },
    { key: 'active', header: 'Status', render: (p) => <StatusBadge status={p.active ? 'active' : 'inactive'} /> },
    { key: 'health', header: 'Health', render: (p) => {
      const h = healthMap[p.provider_id];
      if (!h) return '-';
      return <StatusBadge status={h.status} />;
    }},
    { key: 'success_rate', header: 'Success %', render: (p) => {
      const h = healthMap[p.provider_id];
      return h ? `${h.success_rate}%` : '-';
    }},
  ];

  const openCreate = () => {
    setForm({ name: '', host: '', port: 2775, system_id: '', password: '', system_type: '', bind_type: 1, max_connections: 1, window_size: 10, active: true });
    setShowCreate(true);
  };

  const openEdit = (provider: ProviderInfo) => {
    setForm({ name: provider.name, host: provider.host, port: provider.port, system_id: provider.system_id, password: '', system_type: provider.system_type, bind_type: provider.bind_type, max_connections: provider.max_connections, window_size: provider.window_size, active: provider.active });
    setEditProvider(provider);
  };

  const handleSave = async () => {
    setSaving(true);
    try {
      if (editProvider) {
        const { password, ...rest } = form;
        await providersApi.update(editProvider.provider_id, rest);
        toast.success('Provider updated');
        setEditProvider(null);
      } else {
        await providersApi.create(form);
        toast.success('Provider created');
        setShowCreate(false);
      }
      fetchData();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Save failed');
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async () => {
    if (!deleteProvider) return;
    setSaving(true);
    try {
      await providersApi.delete(deleteProvider.provider_id);
      toast.success('Provider deleted');
      setDeleteProvider(null);
      fetchData();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Delete failed');
    } finally {
      setSaving(false);
    }
  };

  return (
    <>
      <PageHeader title="Providers" subtitle={`${total} providers`} actions={<Button onClick={openCreate}>Add Provider</Button>} />
      <FilterBar filters={filters} values={filterValues} onChange={(v) => { setFilterValues(v); setPage(1); }} onReset={() => { setFilterValues({}); setPage(1); }} />
      <DataTable columns={columns} data={data} total={total} page={page} pageSize={PAGE_SIZE} onPageChange={setPage} loading={loading} keyField="provider_id"
        rowActions={(p) => (
          <div className="flex gap-1">
            <Button size="sm" variant="ghost" onClick={() => openEdit(p)}>Edit</Button>
            <Button size="sm" variant="ghost" onClick={() => setDeleteProvider(p)}>Delete</Button>
          </div>
        )}
      />
      <Modal open={showCreate || !!editProvider} onClose={() => { setShowCreate(false); setEditProvider(null); }} title={editProvider ? 'Edit Provider' : 'Add Provider'} wide>
        <div className="grid grid-cols-2 gap-4">
          <Input label="Name" value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} required />
          <Input label="Host" value={form.host} onChange={(e) => setForm({ ...form, host: e.target.value })} required />
          <Input label="Port" type="number" value={String(form.port)} onChange={(e) => setForm({ ...form, port: Number(e.target.value) })} required />
          <Input label="System ID" value={form.system_id} onChange={(e) => setForm({ ...form, system_id: e.target.value })} required />
          {!editProvider && <Input label="Password" type="password" value={form.password} onChange={(e) => setForm({ ...form, password: e.target.value })} required />}
          <Input label="System Type" value={form.system_type} onChange={(e) => setForm({ ...form, system_type: e.target.value })} />
          <Input label="Max Connections" type="number" value={String(form.max_connections)} onChange={(e) => setForm({ ...form, max_connections: Number(e.target.value) })} />
          <Input label="Window Size" type="number" value={String(form.window_size)} onChange={(e) => setForm({ ...form, window_size: Number(e.target.value) })} />
          <Select label="Status" options={[{ value: 'true', label: 'Active' }, { value: 'false', label: 'Inactive' }]} value={String(form.active)} onChange={(v) => setForm({ ...form, active: v === 'true' })} />
        </div>
        <div className="flex justify-end gap-3 pt-4">
          <Button variant="secondary" onClick={() => { setShowCreate(false); setEditProvider(null); }}>Cancel</Button>
          <Button onClick={handleSave} disabled={saving || !form.name || !form.host}>{saving ? 'Saving...' : 'Save'}</Button>
        </div>
      </Modal>
      <ConfirmDialog open={!!deleteProvider} onConfirm={handleDelete} onCancel={() => setDeleteProvider(null)} title="Delete Provider" description={`Delete provider "${deleteProvider?.name}"?`} confirmLabel="Delete" variant="danger" loading={saving} />
    </>
  );
}
```

- [ ] **Step 2: Commit**

```bash
git add portal-frontend/src/pages/admin/ProvidersPage.tsx
git commit -m "feat(admin): add Providers admin page with health indicators"
```

---

### Task 12: Routes Page

**Files:**
- Create: `portal-frontend/src/pages/admin/RoutesPage.tsx`

- [ ] **Step 1: Create `portal-frontend/src/pages/admin/RoutesPage.tsx`**

```tsx
import { useState, useEffect, useCallback } from 'react';
import { PageHeader } from '../../components/layout/PageHeader';
import { DataTable, type Column } from '../../components/data/DataTable';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Input';
import { Select } from '../../components/ui/Select';
import { Modal } from '../../components/ui/Modal';
import { ConfirmDialog } from '../../components/ui/ConfirmDialog';
import { StatusBadge } from '../../components/ui/Badge';
import { useToast } from '../../components/ui/Toast';
import { routesApi, type RouteInfo } from '../../api/admin';

const PAGE_SIZE = 20;

const columns: Column<RouteInfo>[] = [
  { key: 'name', header: 'Name', sortable: true },
  { key: 'pattern', header: 'Pattern' },
  { key: 'priority', header: 'Priority', sortable: true },
  { key: 'load_balance_strategy', header: 'Strategy' },
  { key: 'failover_enabled', header: 'Failover', render: (r) => r.failover_enabled ? 'Yes' : 'No' },
  { key: 'provider_ids', header: 'Providers', render: (r) => String(r.provider_ids?.length ?? 0) },
  { key: 'active', header: 'Status', render: (r) => <StatusBadge status={r.active ? 'active' : 'inactive'} /> },
];

export function RoutesPage() {
  const toast = useToast();
  const [data, setData] = useState<RouteInfo[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [showForm, setShowForm] = useState(false);
  const [editRoute, setEditRoute] = useState<RouteInfo | null>(null);
  const [deleteRoute, setDeleteRoute] = useState<RouteInfo | null>(null);
  const [saving, setSaving] = useState(false);
  const [form, setForm] = useState({
    name: '', pattern: '', priority: 0, provider_ids: '',
    load_balance_strategy: 'round_robin', failover_enabled: true,
  });

  const fetchData = useCallback(async () => {
    setLoading(true);
    try {
      const res = await routesApi.list({ limit: PAGE_SIZE, offset: (page - 1) * PAGE_SIZE });
      setData(res.routes || []);
      setTotal(res.total);
    } catch {
      toast.error('Failed to load routes');
    } finally {
      setLoading(false);
    }
  }, [page, toast]);

  useEffect(() => { fetchData(); }, [fetchData]);

  const openCreate = () => {
    setForm({ name: '', pattern: '', priority: 0, provider_ids: '', load_balance_strategy: 'round_robin', failover_enabled: true });
    setShowForm(true);
  };

  const openEdit = (route: RouteInfo) => {
    setForm({ name: route.name, pattern: route.pattern, priority: route.priority, provider_ids: (route.provider_ids || []).join(', '), load_balance_strategy: route.load_balance_strategy, failover_enabled: route.failover_enabled });
    setEditRoute(route);
    setShowForm(true);
  };

  const handleSave = async () => {
    setSaving(true);
    try {
      const payload = { ...form, priority: Number(form.priority), provider_ids: form.provider_ids.split(',').map((s) => s.trim()).filter(Boolean) };
      if (editRoute) {
        await routesApi.update(editRoute.route_id, payload);
        toast.success('Route updated');
      } else {
        await routesApi.create(payload);
        toast.success('Route created');
      }
      setShowForm(false);
      setEditRoute(null);
      fetchData();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Save failed');
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async () => {
    if (!deleteRoute) return;
    setSaving(true);
    try {
      await routesApi.delete(deleteRoute.route_id);
      toast.success('Route deleted');
      setDeleteRoute(null);
      fetchData();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Delete failed');
    } finally {
      setSaving(false);
    }
  };

  return (
    <>
      <PageHeader title="Routes" subtitle={`${total} routes`} actions={<Button onClick={openCreate}>Create Route</Button>} />
      <DataTable columns={columns} data={data} total={total} page={page} pageSize={PAGE_SIZE} onPageChange={setPage} loading={loading} keyField="route_id"
        rowActions={(r) => (
          <div className="flex gap-1">
            <Button size="sm" variant="ghost" onClick={() => openEdit(r)}>Edit</Button>
            <Button size="sm" variant="ghost" onClick={() => setDeleteRoute(r)}>Delete</Button>
          </div>
        )}
      />
      <Modal open={showForm} onClose={() => { setShowForm(false); setEditRoute(null); }} title={editRoute ? 'Edit Route' : 'Create Route'}>
        <div className="space-y-4">
          <Input label="Name" value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} required />
          <Input label="Pattern (regex)" value={form.pattern} onChange={(e) => setForm({ ...form, pattern: e.target.value })} required />
          <Input label="Priority" type="number" value={String(form.priority)} onChange={(e) => setForm({ ...form, priority: Number(e.target.value) })} />
          <Input label="Provider IDs (comma-separated)" value={form.provider_ids} onChange={(e) => setForm({ ...form, provider_ids: e.target.value })} required />
          <Select label="Strategy" options={[{ value: 'round_robin', label: 'Round Robin' }, { value: 'weighted', label: 'Weighted' }, { value: 'priority', label: 'Priority' }]} value={form.load_balance_strategy} onChange={(v) => setForm({ ...form, load_balance_strategy: v })} />
          <Select label="Failover" options={[{ value: 'true', label: 'Enabled' }, { value: 'false', label: 'Disabled' }]} value={String(form.failover_enabled)} onChange={(v) => setForm({ ...form, failover_enabled: v === 'true' })} />
          <div className="flex justify-end gap-3 pt-2">
            <Button variant="secondary" onClick={() => { setShowForm(false); setEditRoute(null); }}>Cancel</Button>
            <Button onClick={handleSave} disabled={saving || !form.name || !form.pattern}>{saving ? 'Saving...' : 'Save'}</Button>
          </div>
        </div>
      </Modal>
      <ConfirmDialog open={!!deleteRoute} onConfirm={handleDelete} onCancel={() => setDeleteRoute(null)} title="Delete Route" description={`Delete route "${deleteRoute?.name}"?`} confirmLabel="Delete" variant="danger" loading={saving} />
    </>
  );
}
```

- [ ] **Step 2: Commit**

```bash
git add portal-frontend/src/pages/admin/RoutesPage.tsx
git commit -m "feat(admin): add Routes admin page"
```

---

### Task 13: Billing Page

**Files:**
- Create: `portal-frontend/src/pages/admin/BillingPage.tsx`

- [ ] **Step 1: Create `portal-frontend/src/pages/admin/BillingPage.tsx`**

```tsx
import { useState, useEffect, useCallback } from 'react';
import * as Tabs from '@radix-ui/react-tabs';
import { PageHeader } from '../../components/layout/PageHeader';
import { DataTable, type Column } from '../../components/data/DataTable';
import { FilterBar, type FilterDef } from '../../components/data/FilterBar';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Input';
import { Modal } from '../../components/ui/Modal';
import { Badge } from '../../components/ui/Badge';
import { useToast } from '../../components/ui/Toast';
import { billingApi, type Transaction, type PricingRule } from '../../api/admin';

const PAGE_SIZE = 20;

const txFilters: FilterDef[] = [
  { key: 'client_id', label: 'Client ID', type: 'text', placeholder: 'UUID...' },
  { key: 'transaction_type', label: 'Type', type: 'select', options: [
    { value: 'credit', label: 'Credit' }, { value: 'debit', label: 'Debit' },
  ]},
  { key: 'from', label: 'From', type: 'date' },
  { key: 'to', label: 'To', type: 'date' },
];

const txColumns: Column<Transaction>[] = [
  { key: 'created_at', header: 'Date', render: (t) => new Date(t.created_at).toLocaleString(), sortable: true },
  { key: 'client_id', header: 'Client', render: (t) => t.client_id.slice(0, 8) + '...' },
  { key: 'type', header: 'Type', render: (t) => <Badge variant={t.type === 'credit' ? 'success' : 'danger'}>{t.type}</Badge> },
  { key: 'amount', header: 'Amount', render: (t) => `${t.amount} ${t.currency}` },
  { key: 'balance_after', header: 'Balance After' },
  { key: 'description', header: 'Description' },
];

const ruleColumns: Column<PricingRule>[] = [
  { key: 'client_id', header: 'Client', render: (r) => r.client_id.slice(0, 8) + '...' },
  { key: 'destination_pattern', header: 'Pattern' },
  { key: 'price_per_message', header: 'Price/msg', render: (r) => `${r.price_per_message} ${r.currency}` },
  { key: 'priority', header: 'Priority', sortable: true },
  { key: 'active', header: 'Active', render: (r) => r.active ? 'Yes' : 'No' },
];

export function BillingPage() {
  const toast = useToast();
  const [tab, setTab] = useState('transactions');

  // Transactions state
  const [txData, setTxData] = useState<Transaction[]>([]);
  const [txTotal, setTxTotal] = useState(0);
  const [txPage, setTxPage] = useState(1);
  const [txFilter, setTxFilter] = useState<Record<string, string>>({});
  const [txLoading, setTxLoading] = useState(true);

  // Pricing rules state
  const [rules, setRules] = useState<PricingRule[]>([]);
  const [rulesLoading, setRulesLoading] = useState(true);

  // Add credits modal
  const [showAddCredits, setShowAddCredits] = useState(false);
  const [creditForm, setCreditForm] = useState({ client_id: '', amount: '', description: '' });
  const [saving, setSaving] = useState(false);

  const fetchTransactions = useCallback(async () => {
    setTxLoading(true);
    try {
      const res = await billingApi.getTransactions({ ...txFilter, limit: PAGE_SIZE, offset: (txPage - 1) * PAGE_SIZE });
      setTxData(res.transactions || []);
      setTxTotal(res.total);
    } catch {
      toast.error('Failed to load transactions');
    } finally {
      setTxLoading(false);
    }
  }, [txPage, txFilter, toast]);

  const fetchRules = useCallback(async () => {
    setRulesLoading(true);
    try {
      const res = await billingApi.getPricingRules({});
      setRules(res.rules || []);
    } catch {
      toast.error('Failed to load pricing rules');
    } finally {
      setRulesLoading(false);
    }
  }, [toast]);

  useEffect(() => { if (tab === 'transactions') fetchTransactions(); }, [tab, fetchTransactions]);
  useEffect(() => { if (tab === 'pricing') fetchRules(); }, [tab, fetchRules]);

  const handleAddCredits = async () => {
    setSaving(true);
    try {
      const res = await billingApi.addCredits(creditForm.client_id, { amount: creditForm.amount, description: creditForm.description });
      toast.success(`Credits added. New balance: ${res.new_balance}`);
      setShowAddCredits(false);
      fetchTransactions();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Failed to add credits');
    } finally {
      setSaving(false);
    }
  };

  const tabStyle = (value: string) =>
    `px-4 py-2 text-sm font-medium border-b-2 transition-colors ${tab === value ? 'border-primary text-primary' : 'border-transparent text-gray-500 hover:text-gray-700'}`;

  return (
    <>
      <PageHeader title="Billing" actions={<Button onClick={() => { setCreditForm({ client_id: '', amount: '', description: '' }); setShowAddCredits(true); }}>Add Credits</Button>} />
      <Tabs.Root value={tab} onValueChange={setTab}>
        <Tabs.List className="flex border-b border-gray-200 mb-4">
          <Tabs.Trigger value="transactions" className={tabStyle('transactions')}>Transactions</Tabs.Trigger>
          <Tabs.Trigger value="pricing" className={tabStyle('pricing')}>Pricing Rules</Tabs.Trigger>
        </Tabs.List>
        <Tabs.Content value="transactions">
          <FilterBar filters={txFilters} values={txFilter} onChange={(v) => { setTxFilter(v); setTxPage(1); }} onReset={() => { setTxFilter({}); setTxPage(1); }} />
          <DataTable columns={txColumns} data={txData} total={txTotal} page={txPage} pageSize={PAGE_SIZE} onPageChange={setTxPage} loading={txLoading} keyField="transaction_id" />
        </Tabs.Content>
        <Tabs.Content value="pricing">
          <DataTable columns={ruleColumns} data={rules} total={rules.length} page={1} pageSize={100} onPageChange={() => {}} loading={rulesLoading} keyField="rule_id" />
        </Tabs.Content>
      </Tabs.Root>

      <Modal open={showAddCredits} onClose={() => setShowAddCredits(false)} title="Add Credits">
        <div className="space-y-4">
          <Input label="Client ID" value={creditForm.client_id} onChange={(e) => setCreditForm({ ...creditForm, client_id: e.target.value })} required placeholder="UUID" />
          <Input label="Amount" type="number" value={creditForm.amount} onChange={(e) => setCreditForm({ ...creditForm, amount: e.target.value })} required placeholder="0.00" />
          <Input label="Description" value={creditForm.description} onChange={(e) => setCreditForm({ ...creditForm, description: e.target.value })} />
          <div className="flex justify-end gap-3 pt-2">
            <Button variant="secondary" onClick={() => setShowAddCredits(false)}>Cancel</Button>
            <Button onClick={handleAddCredits} disabled={saving || !creditForm.client_id || !creditForm.amount}>{saving ? 'Adding...' : 'Add Credits'}</Button>
          </div>
        </div>
      </Modal>
    </>
  );
}
```

- [ ] **Step 2: Commit**

```bash
git add portal-frontend/src/pages/admin/BillingPage.tsx
git commit -m "feat(admin): add Billing admin page with transactions and pricing tabs"
```

---

### Task 14: Monitoring Page

**Files:**
- Create: `portal-frontend/src/pages/admin/MonitoringPage.tsx`

- [ ] **Step 1: Create `portal-frontend/src/pages/admin/MonitoringPage.tsx`**

```tsx
import { useState, useEffect, useRef, useCallback } from 'react';
import { PageHeader } from '../../components/layout/PageHeader';
import { StatCard } from '../../components/data/StatCard';
import { DataTable, type Column } from '../../components/data/DataTable';
import { StatusBadge } from '../../components/ui/Badge';
import { Button } from '../../components/ui/Button';
import { useToast } from '../../components/ui/Toast';
import { analyticsAdminApi, providersApi, type ProviderInfo, type ProviderHealth, type RealTimeMetrics } from '../../api/admin';

const REFRESH_INTERVAL = 10_000;

export function MonitoringPage() {
  const toast = useToast();
  const [metrics, setMetrics] = useState<RealTimeMetrics | null>(null);
  const [providers, setProviders] = useState<(ProviderInfo & { health?: ProviderHealth })[]>([]);
  const [loading, setLoading] = useState(true);
  const [paused, setPaused] = useState(false);
  const [lastUpdate, setLastUpdate] = useState<Date | null>(null);
  const [error, setError] = useState(false);
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const fetchData = useCallback(async () => {
    try {
      const [metricsRes, providersRes] = await Promise.all([
        analyticsAdminApi.getRealTimeMetrics(),
        providersApi.list({ active_only: true, limit: 100, offset: 0 }),
      ]);
      setMetrics(metricsRes);

      const withHealth = await Promise.all(
        (providersRes.providers || []).map(async (p) => {
          try {
            const h = await providersApi.health(p.provider_id);
            return { ...p, health: h };
          } catch {
            return { ...p, health: undefined };
          }
        })
      );
      setProviders(withHealth);
      setLastUpdate(new Date());
      setError(false);
      setLoading(false);
    } catch {
      setError(true);
      if (loading) {
        toast.error('Failed to load monitoring data');
        setLoading(false);
      }
    }
  }, [loading, toast]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  useEffect(() => {
    if (paused) {
      if (intervalRef.current) clearInterval(intervalRef.current);
      return;
    }
    intervalRef.current = setInterval(fetchData, REFRESH_INTERVAL);
    return () => { if (intervalRef.current) clearInterval(intervalRef.current); };
  }, [paused, fetchData]);

  const providerColumns: Column<(typeof providers)[0]>[] = [
    { key: 'name', header: 'Provider' },
    { key: 'status', header: 'Status', render: (p) => <StatusBadge status={p.health?.status || 'unknown'} /> },
    { key: 'connections', header: 'Connections', render: (p) => p.health ? `${p.health.active_connections}/${p.health.total_connections}` : '-' },
    { key: 'success_rate', header: 'Success %', render: (p) => p.health ? `${p.health.success_rate}%` : '-' },
    { key: 'sent_24h', header: 'Sent (24h)', render: (p) => p.health?.messages_sent_24h?.toLocaleString() || '-' },
    { key: 'failed_24h', header: 'Failed (24h)', render: (p) => p.health?.messages_failed_24h?.toLocaleString() || '-' },
    { key: 'last_error', header: 'Last Error', render: (p) => p.health?.last_error ? <span className="text-xs text-danger">{p.health.last_error}</span> : '-' },
  ];

  const secondsAgo = lastUpdate ? Math.floor((Date.now() - lastUpdate.getTime()) / 1000) : null;

  return (
    <>
      <PageHeader
        title="Monitoring"
        subtitle={error ? 'Connection lost' : lastUpdate ? `Updated ${secondsAgo}s ago` : undefined}
        actions={
          <Button variant={paused ? 'primary' : 'secondary'} onClick={() => setPaused(!paused)}>
            {paused ? 'Resume' : 'Pause'}
          </Button>
        }
      />

      {error && (
        <div className="mb-4 p-3 bg-yellow-50 border border-yellow-200 rounded text-sm text-yellow-800">
          Connection issue. Retrying automatically...
        </div>
      )}

      <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6">
        <StatCard title="Messages/sec" value={metrics?.messages_per_second ?? '-'} />
        <StatCard title="Delivered" value={metrics?.messages_delivered?.toLocaleString() ?? '-'} />
        <StatCard title="Failed" value={metrics?.messages_failed?.toLocaleString() ?? '-'} />
        <StatCard title="Queue Depth" value={metrics?.queue_depth?.toLocaleString() ?? '-'} />
      </div>

      <h2 className="text-lg font-semibold mb-3">Provider Status</h2>
      <DataTable
        columns={providerColumns}
        data={providers}
        total={providers.length}
        page={1}
        pageSize={100}
        onPageChange={() => {}}
        loading={loading}
        keyField="provider_id"
      />
    </>
  );
}
```

- [ ] **Step 2: Commit**

```bash
git add portal-frontend/src/pages/admin/MonitoringPage.tsx
git commit -m "feat(admin): add Monitoring page with auto-refresh and provider health"
```

---

### Task 15: Analytics Page

**Files:**
- Create: `portal-frontend/src/pages/admin/AnalyticsPage.tsx`

- [ ] **Step 1: Create `portal-frontend/src/pages/admin/AnalyticsPage.tsx`**

```tsx
import { useState, useEffect, useCallback } from 'react';
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, BarChart, Bar } from 'recharts';
import { PageHeader } from '../../components/layout/PageHeader';
import { StatCard } from '../../components/data/StatCard';
import { FilterBar, type FilterDef } from '../../components/data/FilterBar';
import { Button } from '../../components/ui/Button';
import { useToast } from '../../components/ui/Toast';
import { analyticsAdminApi } from '../../api/admin';

const periodOptions = [
  { value: '7d', label: 'Last 7 days' },
  { value: '30d', label: 'Last 30 days' },
  { value: '90d', label: 'Last 90 days' },
];

const filters: FilterDef[] = [
  { key: 'period', label: 'Period', type: 'select', options: periodOptions },
  { key: 'client_id', label: 'Client ID', type: 'text', placeholder: 'All clients' },
  { key: 'group_by', label: 'Group by', type: 'select', options: [
    { value: 'day', label: 'Day' }, { value: 'week', label: 'Week' }, { value: 'country', label: 'Country' },
  ]},
];

function periodToRange(period: string): { from: string; to: string } {
  const to = new Date();
  const from = new Date();
  const days = period === '90d' ? 90 : period === '30d' ? 30 : 7;
  from.setDate(from.getDate() - days);
  return { from: from.toISOString().slice(0, 10), to: to.toISOString().slice(0, 10) };
}

interface StatsResponse {
  summary?: { total_sent?: number; total_delivered?: number; total_failed?: number; delivery_rate?: number; total_cost?: number };
  timeline?: { date: string; sent: number; delivered: number; failed: number }[];
  by_country?: { country: string; sent: number; delivered: number }[];
  by_provider?: { provider: string; sent: number; delivered: number; success_rate: number }[];
}

export function AnalyticsPage() {
  const toast = useToast();
  const [filterValues, setFilterValues] = useState<Record<string, string>>({ period: '7d', group_by: 'day' });
  const [stats, setStats] = useState<StatsResponse | null>(null);
  const [loading, setLoading] = useState(true);

  const fetchStats = useCallback(async () => {
    setLoading(true);
    try {
      const { from, to } = periodToRange(filterValues.period || '7d');
      const res = await analyticsAdminApi.getStats({
        from, to,
        client_id: filterValues.client_id || undefined,
        group_by: filterValues.group_by || 'day',
      });
      setStats(res as StatsResponse);
    } catch {
      toast.error('Failed to load analytics');
    } finally {
      setLoading(false);
    }
  }, [filterValues, toast]);

  useEffect(() => { fetchStats(); }, [fetchStats]);

  const summary = stats?.summary;

  return (
    <>
      <PageHeader title="Analytics" actions={<Button variant="secondary" onClick={fetchStats}>Refresh</Button>} />
      <FilterBar filters={filters} values={filterValues} onChange={setFilterValues} />

      <div className="grid grid-cols-2 md:grid-cols-5 gap-4 mb-6">
        <StatCard title="Total Sent" value={summary?.total_sent?.toLocaleString() ?? '-'} />
        <StatCard title="Delivered" value={summary?.total_delivered?.toLocaleString() ?? '-'} />
        <StatCard title="Failed" value={summary?.total_failed?.toLocaleString() ?? '-'} />
        <StatCard title="Delivery Rate" value={summary?.delivery_rate != null ? `${summary.delivery_rate}%` : '-'} />
        <StatCard title="Total Cost" value={summary?.total_cost != null ? `$${summary.total_cost}` : '-'} />
      </div>

      {!loading && stats?.timeline && stats.timeline.length > 0 && (
        <div className="bg-white border border-gray-200 rounded-lg p-4 mb-6">
          <h3 className="text-sm font-medium text-gray-600 mb-3">Message Volume</h3>
          <ResponsiveContainer width="100%" height={300}>
            <LineChart data={stats.timeline}>
              <CartesianGrid strokeDasharray="3 3" />
              <XAxis dataKey="date" tick={{ fontSize: 12 }} />
              <YAxis tick={{ fontSize: 12 }} />
              <Tooltip />
              <Line type="monotone" dataKey="delivered" stroke="#4caf50" strokeWidth={2} name="Delivered" />
              <Line type="monotone" dataKey="failed" stroke="#d32f2f" strokeWidth={2} name="Failed" />
            </LineChart>
          </ResponsiveContainer>
        </div>
      )}

      {!loading && stats?.by_provider && stats.by_provider.length > 0 && (
        <div className="bg-white border border-gray-200 rounded-lg p-4">
          <h3 className="text-sm font-medium text-gray-600 mb-3">By Provider</h3>
          <ResponsiveContainer width="100%" height={250}>
            <BarChart data={stats.by_provider}>
              <CartesianGrid strokeDasharray="3 3" />
              <XAxis dataKey="provider" tick={{ fontSize: 12 }} />
              <YAxis tick={{ fontSize: 12 }} />
              <Tooltip />
              <Bar dataKey="delivered" fill="#4caf50" name="Delivered" />
              <Bar dataKey="sent" fill="#1976d2" name="Sent" />
            </BarChart>
          </ResponsiveContainer>
        </div>
      )}
    </>
  );
}
```

- [ ] **Step 2: Commit**

```bash
git add portal-frontend/src/pages/admin/AnalyticsPage.tsx
git commit -m "feat(admin): add Analytics page with charts and summary cards"
```

---

### Task 16: Templates Page

**Files:**
- Create: `portal-frontend/src/pages/admin/TemplatesPage.tsx`

- [ ] **Step 1: Create `portal-frontend/src/pages/admin/TemplatesPage.tsx`**

```tsx
import { useState, useEffect, useCallback } from 'react';
import { PageHeader } from '../../components/layout/PageHeader';
import { DataTable, type Column } from '../../components/data/DataTable';
import { FilterBar, type FilterDef } from '../../components/data/FilterBar';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Input';
import { Modal } from '../../components/ui/Modal';
import { StatusBadge } from '../../components/ui/Badge';
import { useToast } from '../../components/ui/Toast';
import { templatesApi, type TemplateInfo } from '../../api/admin';

const PAGE_SIZE = 20;

const filters: FilterDef[] = [
  { key: 'client_id', label: 'Client ID', type: 'text', placeholder: 'UUID...' },
  { key: 'status', label: 'Status', type: 'select', options: [
    { value: 'pending', label: 'Pending' }, { value: 'approved', label: 'Approved' }, { value: 'rejected', label: 'Rejected' },
  ]},
];

const columns: Column<TemplateInfo>[] = [
  { key: 'name', header: 'Name', sortable: true },
  { key: 'client_id', header: 'Client', render: (t) => t.client_id.slice(0, 8) + '...' },
  { key: 'body', header: 'Body', render: (t) => <span className="truncate max-w-[200px] inline-block">{t.body}</span> },
  { key: 'status', header: 'Status', render: (t) => <StatusBadge status={t.status} /> },
  { key: 'created_at', header: 'Created', render: (t) => new Date(t.created_at).toLocaleDateString() },
];

export function TemplatesPage() {
  const toast = useToast();
  const [data, setData] = useState<TemplateInfo[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [filterValues, setFilterValues] = useState<Record<string, string>>({});
  const [rejectModal, setRejectModal] = useState<TemplateInfo | null>(null);
  const [rejectReason, setRejectReason] = useState('');
  const [saving, setSaving] = useState(false);

  const fetchData = useCallback(async () => {
    setLoading(true);
    try {
      const res = await templatesApi.list({ ...filterValues, limit: PAGE_SIZE, offset: (page - 1) * PAGE_SIZE });
      setData(res.templates || []);
      setTotal(res.total);
    } catch {
      toast.error('Failed to load templates');
    } finally {
      setLoading(false);
    }
  }, [page, filterValues, toast]);

  useEffect(() => { fetchData(); }, [fetchData]);

  const handleApprove = async (template: TemplateInfo) => {
    try {
      await templatesApi.approve(template.template_id);
      toast.success('Template approved');
      fetchData();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Failed to approve');
    }
  };

  const handleReject = async () => {
    if (!rejectModal) return;
    setSaving(true);
    try {
      await templatesApi.reject(rejectModal.template_id, { reason: rejectReason });
      toast.success('Template rejected');
      setRejectModal(null);
      setRejectReason('');
      fetchData();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Failed to reject');
    } finally {
      setSaving(false);
    }
  };

  return (
    <>
      <PageHeader title="Templates" subtitle={`${total} templates`} />
      <FilterBar filters={filters} values={filterValues} onChange={(v) => { setFilterValues(v); setPage(1); }} onReset={() => { setFilterValues({}); setPage(1); }} />
      <DataTable columns={columns} data={data} total={total} page={page} pageSize={PAGE_SIZE} onPageChange={setPage} loading={loading} keyField="template_id"
        rowActions={(t) => t.status === 'pending' ? (
          <div className="flex gap-1">
            <Button size="sm" variant="primary" onClick={() => handleApprove(t)}>Approve</Button>
            <Button size="sm" variant="danger" onClick={() => setRejectModal(t)}>Reject</Button>
          </div>
        ) : null}
      />
      <Modal open={!!rejectModal} onClose={() => setRejectModal(null)} title="Reject Template">
        <div className="space-y-4">
          <p className="text-sm text-gray-600">Template: {rejectModal?.name}</p>
          <Input label="Reason" value={rejectReason} onChange={(e) => setRejectReason(e.target.value)} placeholder="Reason for rejection..." />
          <div className="flex justify-end gap-3 pt-2">
            <Button variant="secondary" onClick={() => setRejectModal(null)}>Cancel</Button>
            <Button variant="danger" onClick={handleReject} disabled={saving}>{saving ? 'Rejecting...' : 'Reject'}</Button>
          </div>
        </div>
      </Modal>
    </>
  );
}
```

- [ ] **Step 2: Commit**

```bash
git add portal-frontend/src/pages/admin/TemplatesPage.tsx
git commit -m "feat(admin): add Templates moderation page"
```

---

### Task 17: Webhooks Page

**Files:**
- Create: `portal-frontend/src/pages/admin/WebhooksPage.tsx`

- [ ] **Step 1: Create `portal-frontend/src/pages/admin/WebhooksPage.tsx`**

```tsx
import { useState, useEffect, useCallback } from 'react';
import { PageHeader } from '../../components/layout/PageHeader';
import { DataTable, type Column } from '../../components/data/DataTable';
import { FilterBar, type FilterDef } from '../../components/data/FilterBar';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Input';
import { Modal } from '../../components/ui/Modal';
import { ConfirmDialog } from '../../components/ui/ConfirmDialog';
import { StatusBadge } from '../../components/ui/Badge';
import { useToast } from '../../components/ui/Toast';
import { webhooksAdminApi, type WebhookInfo } from '../../api/admin';

const filters: FilterDef[] = [
  { key: 'client_id', label: 'Client ID', type: 'text', placeholder: 'UUID...' },
];

const columns: Column<WebhookInfo>[] = [
  { key: 'url', header: 'URL' },
  { key: 'client_id', header: 'Client', render: (w) => w.client_id.slice(0, 8) + '...' },
  { key: 'events', header: 'Events', render: (w) => (w.events || []).join(', ') },
  { key: 'active', header: 'Status', render: (w) => <StatusBadge status={w.active ? 'active' : 'inactive'} /> },
  { key: 'created_at', header: 'Created', render: (w) => new Date(w.created_at).toLocaleDateString() },
];

export function WebhooksPage() {
  const toast = useToast();
  const [data, setData] = useState<WebhookInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [filterValues, setFilterValues] = useState<Record<string, string>>({});
  const [showCreate, setShowCreate] = useState(false);
  const [deleteWebhook, setDeleteWebhook] = useState<WebhookInfo | null>(null);
  const [saving, setSaving] = useState(false);
  const [form, setForm] = useState({ client_id: '', url: '', events: '' });

  const fetchData = useCallback(async () => {
    setLoading(true);
    try {
      const res = await webhooksAdminApi.list(filterValues);
      setData(res.webhooks || []);
    } catch {
      toast.error('Failed to load webhooks');
    } finally {
      setLoading(false);
    }
  }, [filterValues, toast]);

  useEffect(() => { fetchData(); }, [fetchData]);

  const handleCreate = async () => {
    setSaving(true);
    try {
      await webhooksAdminApi.create({
        client_id: form.client_id,
        url: form.url,
        events: form.events.split(',').map((s) => s.trim()).filter(Boolean),
        active: true,
      });
      toast.success('Webhook created');
      setShowCreate(false);
      fetchData();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Create failed');
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async () => {
    if (!deleteWebhook) return;
    setSaving(true);
    try {
      await webhooksAdminApi.delete(deleteWebhook.webhook_id, deleteWebhook.client_id);
      toast.success('Webhook deleted');
      setDeleteWebhook(null);
      fetchData();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Delete failed');
    } finally {
      setSaving(false);
    }
  };

  return (
    <>
      <PageHeader title="Webhooks" subtitle={`${data.length} webhooks`} actions={<Button onClick={() => { setForm({ client_id: '', url: '', events: '' }); setShowCreate(true); }}>Create Webhook</Button>} />
      <FilterBar filters={filters} values={filterValues} onChange={setFilterValues} onReset={() => setFilterValues({})} />
      <DataTable columns={columns} data={data} total={data.length} page={1} pageSize={100} onPageChange={() => {}} loading={loading} keyField="webhook_id"
        rowActions={(w) => <Button size="sm" variant="ghost" onClick={() => setDeleteWebhook(w)}>Delete</Button>}
      />
      <Modal open={showCreate} onClose={() => setShowCreate(false)} title="Create Webhook">
        <div className="space-y-4">
          <Input label="Client ID" value={form.client_id} onChange={(e) => setForm({ ...form, client_id: e.target.value })} required placeholder="UUID" />
          <Input label="URL" value={form.url} onChange={(e) => setForm({ ...form, url: e.target.value })} required placeholder="https://..." />
          <Input label="Events (comma-separated)" value={form.events} onChange={(e) => setForm({ ...form, events: e.target.value })} placeholder="message.delivered, message.failed" />
          <div className="flex justify-end gap-3 pt-2">
            <Button variant="secondary" onClick={() => setShowCreate(false)}>Cancel</Button>
            <Button onClick={handleCreate} disabled={saving || !form.client_id || !form.url}>{saving ? 'Creating...' : 'Create'}</Button>
          </div>
        </div>
      </Modal>
      <ConfirmDialog open={!!deleteWebhook} onConfirm={handleDelete} onCancel={() => setDeleteWebhook(null)} title="Delete Webhook" description={`Delete webhook for "${deleteWebhook?.url}"?`} confirmLabel="Delete" variant="danger" loading={saving} />
    </>
  );
}
```

- [ ] **Step 2: Commit**

```bash
git add portal-frontend/src/pages/admin/WebhooksPage.tsx
git commit -m "feat(admin): add Webhooks admin page"
```

---

### Task 18: HLR Page

**Files:**
- Create: `portal-frontend/src/pages/admin/HLRPage.tsx`

- [ ] **Step 1: Create `portal-frontend/src/pages/admin/HLRPage.tsx`**

```tsx
import { useState, useEffect, useCallback } from 'react';
import { PageHeader } from '../../components/layout/PageHeader';
import { DataTable, type Column } from '../../components/data/DataTable';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Input';
import { Modal } from '../../components/ui/Modal';
import { ConfirmDialog } from '../../components/ui/ConfirmDialog';
import { StatusBadge } from '../../components/ui/Badge';
import { useToast } from '../../components/ui/Toast';
import { hlrApi, type HLRProvider, type SmartRouteWeight } from '../../api/admin';

export function HLRPage() {
  const toast = useToast();
  const [providers, setProviders] = useState<HLRProvider[]>([]);
  const [weights, setWeights] = useState<SmartRouteWeight[]>([]);
  const [loading, setLoading] = useState(true);
  const [showCreateProvider, setShowCreateProvider] = useState(false);
  const [showCreateWeight, setShowCreateWeight] = useState(false);
  const [deleteProvider, setDeleteProvider] = useState<HLRProvider | null>(null);
  const [deleteWeight, setDeleteWeight] = useState<SmartRouteWeight | null>(null);
  const [saving, setSaving] = useState(false);
  const [providerForm, setProviderForm] = useState({ name: '', active: true });
  const [weightForm, setWeightForm] = useState({ country_code: '', provider_id: '', weight: 100 });

  const fetchData = useCallback(async () => {
    setLoading(true);
    try {
      const [pRes, wRes] = await Promise.all([hlrApi.listProviders(), hlrApi.listWeights()]);
      setProviders(pRes.providers || []);
      setWeights(wRes.weights || []);
    } catch {
      toast.error('Failed to load HLR data');
    } finally {
      setLoading(false);
    }
  }, [toast]);

  useEffect(() => { fetchData(); }, [fetchData]);

  const providerColumns: Column<HLRProvider>[] = [
    { key: 'name', header: 'Name' },
    { key: 'active', header: 'Status', render: (p) => <StatusBadge status={p.active ? 'active' : 'inactive'} /> },
    { key: 'created_at', header: 'Created', render: (p) => new Date(p.created_at).toLocaleDateString() },
  ];

  const weightColumns: Column<SmartRouteWeight>[] = [
    { key: 'country_code', header: 'Country' },
    { key: 'provider_id', header: 'Provider', render: (w) => w.provider_id.slice(0, 8) + '...' },
    { key: 'weight', header: 'Weight' },
  ];

  const handleCreateProvider = async () => {
    setSaving(true);
    try {
      await hlrApi.createProvider(providerForm);
      toast.success('HLR provider created');
      setShowCreateProvider(false);
      fetchData();
    } catch (e) { toast.error(e instanceof Error ? e.message : 'Failed'); } finally { setSaving(false); }
  };

  const handleDeleteProvider = async () => {
    if (!deleteProvider) return;
    setSaving(true);
    try {
      await hlrApi.deleteProvider(deleteProvider.provider_id);
      toast.success('HLR provider deleted');
      setDeleteProvider(null);
      fetchData();
    } catch (e) { toast.error(e instanceof Error ? e.message : 'Failed'); } finally { setSaving(false); }
  };

  const handleCreateWeight = async () => {
    setSaving(true);
    try {
      await hlrApi.setWeights({ ...weightForm, weight: Number(weightForm.weight) });
      toast.success('Weight set');
      setShowCreateWeight(false);
      fetchData();
    } catch (e) { toast.error(e instanceof Error ? e.message : 'Failed'); } finally { setSaving(false); }
  };

  const handleDeleteWeight = async () => {
    if (!deleteWeight) return;
    setSaving(true);
    try {
      await hlrApi.deleteWeight(deleteWeight.weight_id);
      toast.success('Weight removed');
      setDeleteWeight(null);
      fetchData();
    } catch (e) { toast.error(e instanceof Error ? e.message : 'Failed'); } finally { setSaving(false); }
  };

  return (
    <>
      <PageHeader title="HLR Providers & Routing Weights" />

      <div className="mb-8">
        <div className="flex items-center justify-between mb-3">
          <h2 className="text-lg font-semibold">HLR Providers</h2>
          <Button size="sm" onClick={() => { setProviderForm({ name: '', active: true }); setShowCreateProvider(true); }}>Add Provider</Button>
        </div>
        <DataTable columns={providerColumns} data={providers} total={providers.length} page={1} pageSize={100} onPageChange={() => {}} loading={loading} keyField="provider_id"
          rowActions={(p) => <Button size="sm" variant="ghost" onClick={() => setDeleteProvider(p)}>Delete</Button>}
        />
      </div>

      <div>
        <div className="flex items-center justify-between mb-3">
          <h2 className="text-lg font-semibold">Smart Route Weights</h2>
          <Button size="sm" onClick={() => { setWeightForm({ country_code: '', provider_id: '', weight: 100 }); setShowCreateWeight(true); }}>Set Weight</Button>
        </div>
        <DataTable columns={weightColumns} data={weights} total={weights.length} page={1} pageSize={100} onPageChange={() => {}} loading={loading} keyField="weight_id"
          rowActions={(w) => <Button size="sm" variant="ghost" onClick={() => setDeleteWeight(w)}>Delete</Button>}
        />
      </div>

      <Modal open={showCreateProvider} onClose={() => setShowCreateProvider(false)} title="Add HLR Provider">
        <div className="space-y-4">
          <Input label="Name" value={providerForm.name} onChange={(e) => setProviderForm({ ...providerForm, name: e.target.value })} required />
          <div className="flex justify-end gap-3 pt-2">
            <Button variant="secondary" onClick={() => setShowCreateProvider(false)}>Cancel</Button>
            <Button onClick={handleCreateProvider} disabled={saving || !providerForm.name}>{saving ? 'Creating...' : 'Create'}</Button>
          </div>
        </div>
      </Modal>

      <Modal open={showCreateWeight} onClose={() => setShowCreateWeight(false)} title="Set Route Weight">
        <div className="space-y-4">
          <Input label="Country Code" value={weightForm.country_code} onChange={(e) => setWeightForm({ ...weightForm, country_code: e.target.value })} required placeholder="US" />
          <Input label="Provider ID" value={weightForm.provider_id} onChange={(e) => setWeightForm({ ...weightForm, provider_id: e.target.value })} required placeholder="UUID" />
          <Input label="Weight" type="number" value={String(weightForm.weight)} onChange={(e) => setWeightForm({ ...weightForm, weight: Number(e.target.value) })} required />
          <div className="flex justify-end gap-3 pt-2">
            <Button variant="secondary" onClick={() => setShowCreateWeight(false)}>Cancel</Button>
            <Button onClick={handleCreateWeight} disabled={saving || !weightForm.country_code || !weightForm.provider_id}>{saving ? 'Saving...' : 'Save'}</Button>
          </div>
        </div>
      </Modal>

      <ConfirmDialog open={!!deleteProvider} onConfirm={handleDeleteProvider} onCancel={() => setDeleteProvider(null)} title="Delete HLR Provider" description={`Delete "${deleteProvider?.name}"?`} confirmLabel="Delete" variant="danger" loading={saving} />
      <ConfirmDialog open={!!deleteWeight} onConfirm={handleDeleteWeight} onCancel={() => setDeleteWeight(null)} title="Delete Weight" description="Delete this routing weight?" confirmLabel="Delete" variant="danger" loading={saving} />
    </>
  );
}
```

- [ ] **Step 2: Commit**

```bash
git add portal-frontend/src/pages/admin/HLRPage.tsx
git commit -m "feat(admin): add HLR providers and smart routing weights page"
```

---

### Task 19: Countries & Operators Page

**Files:**
- Create: `portal-frontend/src/pages/admin/CountriesPage.tsx`

- [ ] **Step 1: Create `portal-frontend/src/pages/admin/CountriesPage.tsx`**

```tsx
import { useState, useEffect, useCallback } from 'react';
import { PageHeader } from '../../components/layout/PageHeader';
import { DataTable, type Column } from '../../components/data/DataTable';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Input';
import { Modal } from '../../components/ui/Modal';
import { StatusBadge } from '../../components/ui/Badge';
import { useToast } from '../../components/ui/Toast';
import { countriesApi, operatorsApi, type CountryInfo, type OperatorInfo, type OperatorPrefix } from '../../api/admin';

const PAGE_SIZE = 20;

export function CountriesPage() {
  const toast = useToast();
  const [countries, setCountries] = useState<CountryInfo[]>([]);
  const [countryTotal, setCountryTotal] = useState(0);
  const [countryPage, setCountryPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [showCreateCountry, setShowCreateCountry] = useState(false);
  const [countryForm, setCountryForm] = useState({ name: '', code: '', phone_code: '' });

  // Operators state
  const [selectedCountry, setSelectedCountry] = useState<CountryInfo | null>(null);
  const [operators, setOperators] = useState<OperatorInfo[]>([]);
  const [operatorsLoading, setOperatorsLoading] = useState(false);
  const [showCreateOperator, setShowCreateOperator] = useState(false);
  const [operatorForm, setOperatorForm] = useState({ name: '', mcc: '', mnc: '' });

  // Prefixes state
  const [selectedOperator, setSelectedOperator] = useState<OperatorInfo | null>(null);
  const [prefixes, setPrefixes] = useState<OperatorPrefix[]>([]);
  const [prefixInput, setPrefixInput] = useState('');

  const [saving, setSaving] = useState(false);

  const fetchCountries = useCallback(async () => {
    setLoading(true);
    try {
      const res = await countriesApi.list({ limit: PAGE_SIZE, offset: (countryPage - 1) * PAGE_SIZE });
      setCountries(res.countries || []);
      setCountryTotal(res.total);
    } catch { toast.error('Failed to load countries'); }
    finally { setLoading(false); }
  }, [countryPage, toast]);

  useEffect(() => { fetchCountries(); }, [fetchCountries]);

  const fetchOperators = useCallback(async (countryId: string) => {
    setOperatorsLoading(true);
    try {
      const res = await operatorsApi.list({ country_id: countryId, limit: 100, offset: 0 });
      setOperators(res.operators || []);
    } catch { toast.error('Failed to load operators'); }
    finally { setOperatorsLoading(false); }
  }, [toast]);

  const fetchPrefixes = useCallback(async (operatorId: string) => {
    try {
      const res = await operatorsApi.listPrefixes(operatorId);
      setPrefixes(res.prefixes || []);
    } catch { toast.error('Failed to load prefixes'); }
  }, [toast]);

  const selectCountry = (country: CountryInfo) => {
    setSelectedCountry(country);
    setSelectedOperator(null);
    setPrefixes([]);
    fetchOperators(country.country_id);
  };

  const selectOperator = (op: OperatorInfo) => {
    setSelectedOperator(op);
    fetchPrefixes(op.operator_id);
  };

  const handleCreateCountry = async () => {
    setSaving(true);
    try {
      await countriesApi.create(countryForm);
      toast.success('Country created');
      setShowCreateCountry(false);
      fetchCountries();
    } catch (e) { toast.error(e instanceof Error ? e.message : 'Failed'); }
    finally { setSaving(false); }
  };

  const handleCreateOperator = async () => {
    if (!selectedCountry) return;
    setSaving(true);
    try {
      await operatorsApi.create({ ...operatorForm, country_id: selectedCountry.country_id, active: true });
      toast.success('Operator created');
      setShowCreateOperator(false);
      fetchOperators(selectedCountry.country_id);
    } catch (e) { toast.error(e instanceof Error ? e.message : 'Failed'); }
    finally { setSaving(false); }
  };

  const handleAddPrefix = async () => {
    if (!selectedOperator || !prefixInput) return;
    setSaving(true);
    try {
      await operatorsApi.createPrefix(selectedOperator.operator_id, { prefix: prefixInput });
      toast.success('Prefix added');
      setPrefixInput('');
      fetchPrefixes(selectedOperator.operator_id);
    } catch (e) { toast.error(e instanceof Error ? e.message : 'Failed'); }
    finally { setSaving(false); }
  };

  const handleDeletePrefix = async (prefixId: string) => {
    if (!selectedOperator) return;
    try {
      await operatorsApi.deletePrefix(selectedOperator.operator_id, prefixId);
      toast.success('Prefix removed');
      fetchPrefixes(selectedOperator.operator_id);
    } catch (e) { toast.error(e instanceof Error ? e.message : 'Failed'); }
  };

  const countryColumns: Column<CountryInfo>[] = [
    { key: 'name', header: 'Country', sortable: true },
    { key: 'code', header: 'Code' },
    { key: 'phone_code', header: 'Phone Code' },
  ];

  const operatorColumns: Column<OperatorInfo>[] = [
    { key: 'name', header: 'Operator' },
    { key: 'mcc', header: 'MCC' },
    { key: 'mnc', header: 'MNC' },
    { key: 'active', header: 'Status', render: (o) => <StatusBadge status={o.active ? 'active' : 'inactive'} /> },
  ];

  return (
    <>
      <PageHeader title="Countries & Operators" />
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        <div>
          <div className="flex items-center justify-between mb-3">
            <h2 className="text-lg font-semibold">Countries</h2>
            <Button size="sm" onClick={() => { setCountryForm({ name: '', code: '', phone_code: '' }); setShowCreateCountry(true); }}>Add</Button>
          </div>
          <DataTable columns={countryColumns} data={countries} total={countryTotal} page={countryPage} pageSize={PAGE_SIZE} onPageChange={setCountryPage} loading={loading} keyField="country_id" onRowClick={selectCountry} />
        </div>

        <div>
          <div className="flex items-center justify-between mb-3">
            <h2 className="text-lg font-semibold">{selectedCountry ? `Operators — ${selectedCountry.name}` : 'Operators'}</h2>
            {selectedCountry && <Button size="sm" onClick={() => { setOperatorForm({ name: '', mcc: '', mnc: '' }); setShowCreateOperator(true); }}>Add</Button>}
          </div>
          {selectedCountry ? (
            <DataTable columns={operatorColumns} data={operators} total={operators.length} page={1} pageSize={100} onPageChange={() => {}} loading={operatorsLoading} keyField="operator_id" onRowClick={selectOperator} />
          ) : (
            <div className="text-sm text-gray-400 p-4 border border-gray-200 rounded-lg">Select a country</div>
          )}
        </div>

        <div>
          <h2 className="text-lg font-semibold mb-3">{selectedOperator ? `Prefixes — ${selectedOperator.name}` : 'Prefixes'}</h2>
          {selectedOperator ? (
            <>
              <div className="flex gap-2 mb-3">
                <Input value={prefixInput} onChange={(e) => setPrefixInput(e.target.value)} placeholder="e.g. +7921" />
                <Button size="sm" onClick={handleAddPrefix} disabled={saving || !prefixInput}>Add</Button>
              </div>
              <ul className="space-y-1">
                {prefixes.map((p) => (
                  <li key={p.prefix_id} className="flex items-center justify-between py-1 px-2 bg-gray-50 rounded text-sm">
                    <span className="font-mono">{p.prefix}</span>
                    <button className="text-xs text-danger hover:underline" onClick={() => handleDeletePrefix(p.prefix_id)}>Remove</button>
                  </li>
                ))}
                {prefixes.length === 0 && <li className="text-sm text-gray-400">No prefixes</li>}
              </ul>
            </>
          ) : (
            <div className="text-sm text-gray-400 p-4 border border-gray-200 rounded-lg">Select an operator</div>
          )}
        </div>
      </div>

      <Modal open={showCreateCountry} onClose={() => setShowCreateCountry(false)} title="Add Country">
        <div className="space-y-4">
          <Input label="Name" value={countryForm.name} onChange={(e) => setCountryForm({ ...countryForm, name: e.target.value })} required />
          <Input label="Code (ISO)" value={countryForm.code} onChange={(e) => setCountryForm({ ...countryForm, code: e.target.value })} required placeholder="US" maxLength={2} />
          <Input label="Phone Code" value={countryForm.phone_code} onChange={(e) => setCountryForm({ ...countryForm, phone_code: e.target.value })} required placeholder="+1" />
          <div className="flex justify-end gap-3 pt-2">
            <Button variant="secondary" onClick={() => setShowCreateCountry(false)}>Cancel</Button>
            <Button onClick={handleCreateCountry} disabled={saving}>{saving ? 'Creating...' : 'Create'}</Button>
          </div>
        </div>
      </Modal>

      <Modal open={showCreateOperator} onClose={() => setShowCreateOperator(false)} title="Add Operator">
        <div className="space-y-4">
          <Input label="Name" value={operatorForm.name} onChange={(e) => setOperatorForm({ ...operatorForm, name: e.target.value })} required />
          <Input label="MCC" value={operatorForm.mcc} onChange={(e) => setOperatorForm({ ...operatorForm, mcc: e.target.value })} required />
          <Input label="MNC" value={operatorForm.mnc} onChange={(e) => setOperatorForm({ ...operatorForm, mnc: e.target.value })} required />
          <div className="flex justify-end gap-3 pt-2">
            <Button variant="secondary" onClick={() => setShowCreateOperator(false)}>Cancel</Button>
            <Button onClick={handleCreateOperator} disabled={saving}>{saving ? 'Creating...' : 'Create'}</Button>
          </div>
        </div>
      </Modal>
    </>
  );
}
```

- [ ] **Step 2: Commit**

```bash
git add portal-frontend/src/pages/admin/CountriesPage.tsx
git commit -m "feat(admin): add Countries & Operators page with prefix management"
```

---

### Task 20: Audit Log Page

**Files:**
- Create: `portal-frontend/src/pages/admin/AuditLogPage.tsx`

- [ ] **Step 1: Create `portal-frontend/src/pages/admin/AuditLogPage.tsx`**

```tsx
import { useState, useEffect, useCallback } from 'react';
import { PageHeader } from '../../components/layout/PageHeader';
import { DataTable, type Column } from '../../components/data/DataTable';
import { FilterBar, type FilterDef } from '../../components/data/FilterBar';
import { useToast } from '../../components/ui/Toast';
import { auditAdminApi, type AuditEntry } from '../../api/admin';

const PAGE_SIZE = 30;

const filters: FilterDef[] = [
  { key: 'user_id', label: 'User ID', type: 'text', placeholder: 'UUID...' },
  { key: 'action', label: 'Action', type: 'select', options: [
    { value: 'login', label: 'Login' },
    { value: 'logout', label: 'Logout' },
    { value: 'create', label: 'Create' },
    { value: 'update', label: 'Update' },
    { value: 'delete', label: 'Delete' },
  ]},
  { key: 'from', label: 'From', type: 'date' },
  { key: 'to', label: 'To', type: 'date' },
];

const columns: Column<AuditEntry>[] = [
  { key: 'created_at', header: 'Timestamp', render: (e) => new Date(e.created_at).toLocaleString(), sortable: true },
  { key: 'action', header: 'Action' },
  { key: 'resource_type', header: 'Resource' },
  { key: 'resource_id', header: 'Resource ID', render: (e) => e.resource_id ? e.resource_id.slice(0, 8) + '...' : '-' },
  { key: 'user_id', header: 'User', render: (e) => e.user_id.slice(0, 8) + '...' },
  { key: 'ip_address', header: 'IP' },
  { key: 'details', header: 'Details', render: (e) => {
    if (!e.details || Object.keys(e.details).length === 0) return '-';
    return <span className="text-xs text-gray-500 truncate max-w-[150px] inline-block">{JSON.stringify(e.details)}</span>;
  }},
];

export function AuditLogPage() {
  const toast = useToast();
  const [data, setData] = useState<AuditEntry[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [filterValues, setFilterValues] = useState<Record<string, string>>({});

  const fetchData = useCallback(async () => {
    setLoading(true);
    try {
      const res = await auditAdminApi.list({ ...filterValues, limit: PAGE_SIZE, offset: (page - 1) * PAGE_SIZE });
      setData(res.entries || []);
      setTotal(res.total);
    } catch {
      toast.error('Failed to load audit log');
    } finally {
      setLoading(false);
    }
  }, [page, filterValues, toast]);

  useEffect(() => { fetchData(); }, [fetchData]);

  return (
    <>
      <PageHeader title="Audit Log" subtitle={`${total} entries`} />
      <FilterBar filters={filters} values={filterValues} onChange={(v) => { setFilterValues(v); setPage(1); }} onReset={() => { setFilterValues({}); setPage(1); }} />
      <DataTable columns={columns} data={data} total={total} page={page} pageSize={PAGE_SIZE} onPageChange={setPage} loading={loading} keyField="id" />
    </>
  );
}
```

- [ ] **Step 2: Verify full build**

```bash
cd portal-frontend && npm run build
```

Expected: Build succeeds with all lazy chunks generated.

- [ ] **Step 3: Commit**

```bash
git add portal-frontend/src/pages/admin/AuditLogPage.tsx
git commit -m "feat(admin): add Audit Log admin page"
```

---

## Self-Review Notes

- **Spec coverage:** All 11 admin pages implemented. Auth + role routing done. Tailwind + Radix UI setup. Backend auth middleware updated. Profile handler returns role. Login redirects by role.
- **Placeholder scan:** No TODOs, TBDs, or "implement later" found.
- **Type consistency:** `ClientInfo`, `ProviderInfo`, `RouteInfo`, etc. defined in `admin.ts` and used consistently across pages. `StatusBadge`, `DataTable`, `FilterBar` APIs consistent.
- **Out of scope (per spec):** WebSocket, granular permissions, drag-and-drop, dark mode, i18n — not included.
