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
