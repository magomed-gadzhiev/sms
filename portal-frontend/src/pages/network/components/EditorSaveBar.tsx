import { Button } from '../../../components/ui/Button';

function pluralize(n: number): string {
  const mod10 = n % 10;
  const mod100 = n % 100;
  if (mod10 === 1 && mod100 !== 11) return 'изменение';
  if (mod10 >= 2 && mod10 <= 4 && (mod100 < 10 || mod100 >= 20)) return 'изменения';
  return 'изменений';
}

interface Props {
  dirtyCount: number;
  saving: boolean;
  onSave: () => void;
  onCancel: () => void;
}

export function EditorSaveBar({ dirtyCount, saving, onSave, onCancel }: Props) {
  const visible = dirtyCount > 0;
  return (
    <div
      className={`sticky bottom-4 z-30 mt-4 transition-opacity duration-150 ${
        visible ? 'opacity-100 pointer-events-auto' : 'opacity-0 pointer-events-none'
      }`}
      role="region"
      aria-label="Несохранённые изменения"
      aria-hidden={!visible}
    >
      <div className="mx-auto max-w-3xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-lg shadow-lg flex items-center gap-3 px-4 py-2.5">
        <span className="text-sm text-slate-700 dark:text-slate-300">
          Не сохранено: <strong>{dirtyCount}</strong>{' '}
          {pluralize(dirtyCount)}
        </span>
        <div className="ml-auto flex items-center gap-2">
          <Button variant="secondary" disabled={saving} onClick={onCancel}>
            Отменить
          </Button>
          <Button disabled={saving} onClick={onSave}>
            {saving ? 'Сохранение…' : 'Сохранить'}
          </Button>
        </div>
      </div>
    </div>
  );
}
