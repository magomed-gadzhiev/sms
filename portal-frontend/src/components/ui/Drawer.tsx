import { type ReactNode } from 'react';
import * as Dialog from '@radix-ui/react-dialog';

interface DrawerProps {
  open: boolean;
  onClose: () => void;
  title: string;
  description?: string;
  width?: 'sm' | 'md' | 'lg';
  children: ReactNode;
}

const widthClass = {
  sm: 'w-[420px]',
  md: 'w-[560px]',
  lg: 'w-[720px]',
};

export function Drawer({ open, onClose, title, description, width = 'md', children }: DrawerProps) {
  return (
    <Dialog.Root open={open} onOpenChange={(o) => !o && onClose()}>
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 bg-black/40 z-40" />
        <Dialog.Content
          className={`fixed top-0 right-0 h-full ${widthClass[width]} max-w-[90vw]
            bg-white shadow-xl z-50 flex flex-col
            data-[state=open]:animate-in data-[state=closed]:animate-out`}
        >
          <div className="px-6 py-4 border-b border-gray-200 flex items-start justify-between">
            <div>
              <Dialog.Title className="text-lg font-semibold">{title}</Dialog.Title>
              {description && (
                <Dialog.Description className="text-sm text-gray-500 mt-1">{description}</Dialog.Description>
              )}
            </div>
            <Dialog.Close asChild>
              <button className="text-gray-400 hover:text-gray-600" aria-label="Закрыть">✕</button>
            </Dialog.Close>
          </div>
          <div className="flex-1 overflow-y-auto px-6 py-4">{children}</div>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
