import * as Dialog from '@radix-ui/react-dialog';
import { useState } from 'react';
import { Button } from '../ui/Button';

export interface BulkAction<T> {
  label: string;
  variant?: 'primary' | 'danger' | 'secondary';
  requiresConfirmation?: boolean;
  confirmMessage?: (count: number) => string;
  onAction: (ids: string[], items: T[]) => Promise<void> | void;
}

interface Props<T> {
  selectedIds: Set<string>;
  selectedItems: T[];
  actions: BulkAction<T>[];
  onClear: () => void;
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
export function BulkActionBar<T extends Record<string, any>>({
  selectedIds,
  selectedItems,
  actions,
  onClear,
}: Props<T>) {
  const [pendingAction, setPendingAction] = useState<BulkAction<T> | null>(null);
  const [running, setRunning] = useState(false);

  if (selectedIds.size === 0) return null;

  const handleAction = async (action: BulkAction<T>) => {
    if (action.requiresConfirmation) {
      setPendingAction(action);
      return;
    }
    setRunning(true);
    try {
      await action.onAction(Array.from(selectedIds), selectedItems);
      onClear();
    } finally {
      setRunning(false);
    }
  };

  const handleConfirm = async () => {
    if (!pendingAction) return;
    setRunning(true);
    try {
      await pendingAction.onAction(Array.from(selectedIds), selectedItems);
      onClear();
    } finally {
      setRunning(false);
      setPendingAction(null);
    }
  };

  return (
    <>
      <div
        className="flex items-center gap-3 px-4 py-2.5 bg-blue-50 border border-blue-200 rounded-lg mb-3"
        role="toolbar"
        aria-label="Массовые действия"
      >
        <span className="text-sm font-medium text-blue-800">
          Выбрано: {selectedIds.size}
        </span>
        <div className="flex items-center gap-2 ml-auto">
          {actions.map((action) => (
            <Button
              key={action.label}
              size="sm"
              variant={action.variant === 'danger' ? 'danger' : action.variant === 'primary' ? 'primary' : 'secondary'}
              onClick={() => handleAction(action)}
              disabled={running}
            >
              {action.label}
            </Button>
          ))}
          <button
            onClick={onClear}
            className="text-sm text-gray-500 hover:text-gray-700 ml-1"
            aria-label="Снять выделение"
          >
            ✕
          </button>
        </div>
      </div>

      <Dialog.Root open={!!pendingAction} onOpenChange={(open) => !open && setPendingAction(null)}>
        <Dialog.Portal>
          <Dialog.Overlay className="fixed inset-0 bg-black/40 z-50" />
          <Dialog.Content className="fixed top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 bg-white rounded-lg shadow-xl z-50 p-6 w-full max-w-sm">
            <Dialog.Title className="text-base font-semibold text-gray-900 mb-3">
              Подтвердите действие
            </Dialog.Title>
            <Dialog.Description className="text-sm text-gray-600 mb-5">
              {pendingAction?.confirmMessage
                ? pendingAction.confirmMessage(selectedIds.size)
                : `Это действие будет применено к ${selectedIds.size} элементам. Продолжить?`}
            </Dialog.Description>
            <div className="flex justify-end gap-3">
              <Button variant="secondary" onClick={() => setPendingAction(null)} disabled={running}>
                Отмена
              </Button>
              <Button variant="danger" onClick={handleConfirm} disabled={running}>
                {running ? 'Выполняется...' : 'Подтвердить'}
              </Button>
            </div>
          </Dialog.Content>
        </Dialog.Portal>
      </Dialog.Root>
    </>
  );
}
