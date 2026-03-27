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
