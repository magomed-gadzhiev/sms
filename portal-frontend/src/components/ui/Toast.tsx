import { createContext, useContext, useCallback, useMemo, useState, type ReactNode } from 'react';
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
  success: 'border-l-4 border-l-success bg-green-50 dark:bg-green-950/50 dark:border-slate-700',
  error: 'border-l-4 border-l-danger bg-red-50 dark:bg-red-950/50 dark:border-slate-700',
  info: 'border-l-4 border-l-info bg-blue-50 dark:bg-blue-950/50 dark:border-slate-700',
};

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<ToastItem[]>([]);

  const addToast = useCallback((type: ToastItem['type'], message: string) => {
    const id =
      typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function'
        ? crypto.randomUUID()
        : Math.random().toString(36).slice(2) + Date.now().toString(36);
    setToasts((prev) => [...prev, { id, type, message }]);
  }, []);

  const removeToast = useCallback((id: string) => {
    setToasts((prev) => prev.filter((t) => t.id !== id));
  }, []);

  const success = useCallback((m: string) => addToast('success', m), [addToast]);
  const error = useCallback((m: string) => addToast('error', m), [addToast]);
  const info = useCallback((m: string) => addToast('info', m), [addToast]);

  // useMemo критичен: useToast() consumer'ы кладут результат в deps useEffect
  // (например RequireRole/RequireReseller). Без memo каждый setToasts ре-рендерит
  // провайдер, новый литерал value → consumer'ы видят новый ref → их useEffect
  // зацикливается, выстреливая toast снова и снова.
  const value: ToastContextValue = useMemo(() => ({ success, error, info }), [success, error, info]);

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
            <ToastPrimitive.Description className="text-sm text-gray-800 dark:text-slate-100">
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
