import { type ReactNode } from 'react';
import * as Dialog from '@radix-ui/react-dialog';

interface ModalProps {
  open: boolean;
  onClose: () => void;
  title: string;
  description?: string;
  children: ReactNode;
  wide?: boolean;
}

export function Modal({ open, onClose, title, description, children, wide }: ModalProps) {
  return (
    <Dialog.Root open={open} onOpenChange={(o) => !o && onClose()}>
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 bg-black/40 dark:bg-black/70 z-40" />
        <Dialog.Content
          className={`fixed top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2
            bg-white dark:bg-slate-900 text-gray-900 dark:text-slate-100 rounded-lg shadow-xl z-50 p-6 max-h-[85vh] overflow-y-auto
            border border-transparent dark:border-slate-700
            ${wide ? 'w-[90vw] max-w-[700px]' : 'w-[90vw] max-w-[480px]'}`}
        >
          <Dialog.Title className="text-lg font-semibold mb-4">{title}</Dialog.Title>
          <Dialog.Description className="sr-only">
            {description || title}
          </Dialog.Description>
          {children}
          <Dialog.Close asChild>
            <button
              className="absolute top-4 right-4 text-gray-400 dark:text-slate-500 hover:text-gray-600 dark:hover:text-slate-300"
              aria-label="Закрыть"
            >
              ✕
            </button>
          </Dialog.Close>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
