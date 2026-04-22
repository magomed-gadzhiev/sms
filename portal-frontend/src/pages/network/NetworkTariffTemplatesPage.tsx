import * as DropdownMenu from '@radix-ui/react-dropdown-menu';
import { useCallback, useEffect, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import {
  networkTariffsApi,
  type TariffTemplateSummary,
} from '../../api/client';
import { PageHeader } from '../../components/layout/PageHeader';
import { BindTemplateDialog } from './components/BindTemplateDialog';
import { CreateTemplateDialog } from './components/CreateTemplateDialog';
import { DuplicateTemplateDialog } from './components/DuplicateTemplateDialog';

function truncate(s: string | null, max: number): string {
  if (!s) return '';
  if (s.length <= max) return s;
  return s.slice(0, max) + '…';
}

export function NetworkTariffTemplatesPage() {
  const [rows, setRows] = useState<TariffTemplateSummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const nav = useNavigate();

  const [createOpen, setCreateOpen] = useState(false);
  const [bindTarget, setBindTarget] = useState<TariffTemplateSummary | null>(
    null,
  );
  const [duplicateTarget, setDuplicateTarget] =
    useState<TariffTemplateSummary | null>(null);

  const refetch = useCallback(() => {
    let cancelled = false;
    setLoading(true);
    networkTariffsApi
      .listTemplates()
      .then((d) => {
        if (!cancelled) setRows(d);
      })
      .catch((e: unknown) => {
        if (!cancelled) setError(e instanceof Error ? e.message : String(e));
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    const cancel = refetch();
    return cancel;
  }, [refetch]);

  const handleCreate = () => setCreateOpen(true);
  const handleBind = (id: string) => {
    const row = rows.find((r) => r.id === id);
    if (row) setBindTarget(row);
  };
  const handleDuplicate = (id: string) => {
    const row = rows.find((r) => r.id === id);
    if (row) setDuplicateTarget(row);
  };
  const handleDelete = (_id: string) => {
    // stub — delete dialog in a later task
  };

  const headerActions = (
    <>
      <Link
        to="/network/tariffs"
        className="text-sm text-slate-600 hover:text-slate-900 hover:underline self-center"
      >
        ← К субаккаунтам
      </Link>
      <button
        type="button"
        onClick={handleCreate}
        className="inline-flex items-center gap-1 rounded bg-sky-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-sky-700"
      >
        + Создать шаблон
      </button>
    </>
  );

  if (loading) {
    return (
      <div className="max-w-6xl">
        <PageHeader title="Шаблоны тарифов" actions={headerActions} />
        <div className="space-y-2" aria-busy="true" aria-label="Загрузка">
          {[0, 1, 2, 3].map((i) => (
            <div key={i} className="h-10 bg-slate-100 rounded animate-pulse" />
          ))}
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="max-w-6xl">
        <PageHeader title="Шаблоны тарифов" actions={headerActions} />
        <div
          role="alert"
          className="rounded border border-red-200 bg-red-50 text-red-800 px-4 py-3 text-sm"
        >
          Не удалось загрузить шаблоны: {error}
        </div>
      </div>
    );
  }

  return (
    <div className="max-w-6xl">
      <PageHeader title="Шаблоны тарифов" actions={headerActions} />
      {rows.length === 0 ? (
        <div className="text-center py-16 border border-dashed border-slate-200 rounded">
          <p className="text-slate-700 text-sm mb-4 max-w-md mx-auto">
            Создайте первый шаблон. Шаблон — это набор правил, который можно
            применить к нескольким субаккаунтам сразу.
          </p>
          <button
            type="button"
            onClick={handleCreate}
            className="inline-flex items-center gap-1 rounded bg-sky-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-sky-700"
          >
            + Создать шаблон
          </button>
        </div>
      ) : (
        <table className="w-full text-sm">
          <thead className="text-left text-slate-500 bg-slate-50">
            <tr>
              <th className="px-3 py-2 font-medium">Имя</th>
              <th className="px-3 py-2 font-medium">Описание</th>
              <th className="px-3 py-2 font-medium text-right">Планов</th>
              <th className="px-3 py-2 font-medium text-right">Привязано</th>
              <th className="px-3 py-2" aria-label="Действия" />
            </tr>
          </thead>
          <tbody>
            {rows.map((r) => {
              const deleteDisabled = r.bound_subaccount_count > 0;
              return (
                <tr
                  key={r.id}
                  className="border-t border-slate-100 hover:bg-slate-50 cursor-pointer"
                  onClick={() =>
                    nav(`/network/tariffs/editor/${r.id}?mode=template`)
                  }
                >
                  <td className="px-3 py-2">
                    <div className="font-medium text-slate-900">{r.name}</div>
                  </td>
                  <td className="px-3 py-2 text-slate-600">
                    {r.description ? (
                      <span title={r.description}>
                        {truncate(r.description, 60)}
                      </span>
                    ) : (
                      <span className="text-slate-400">—</span>
                    )}
                  </td>
                  <td className="px-3 py-2 text-right tabular-nums">
                    {r.plans_count}
                  </td>
                  <td className="px-3 py-2 text-right tabular-nums">
                    {r.bound_subaccount_count}
                  </td>
                  <td
                    className="px-3 py-2 text-right"
                    onClick={(e) => e.stopPropagation()}
                  >
                    <DropdownMenu.Root>
                      <DropdownMenu.Trigger asChild>
                        <button
                          type="button"
                          aria-label="Действия"
                          className="inline-flex h-7 w-7 items-center justify-center rounded hover:bg-slate-200 text-slate-600"
                        >
                          ⋮
                        </button>
                      </DropdownMenu.Trigger>
                      <DropdownMenu.Portal>
                        <DropdownMenu.Content
                          align="end"
                          sideOffset={4}
                          className="z-50 min-w-40 rounded-md border border-slate-200 bg-white py-1 shadow-lg"
                        >
                          <DropdownMenu.Item
                            className="px-3 py-1.5 text-sm text-slate-700 outline-none cursor-pointer hover:bg-slate-100 data-[highlighted]:bg-slate-100"
                            onSelect={() => handleBind(r.id)}
                          >
                            Привязать…
                          </DropdownMenu.Item>
                          <DropdownMenu.Item
                            className="px-3 py-1.5 text-sm text-slate-700 outline-none cursor-pointer hover:bg-slate-100 data-[highlighted]:bg-slate-100"
                            onSelect={() => handleDuplicate(r.id)}
                          >
                            Дублировать
                          </DropdownMenu.Item>
                          <DropdownMenu.Item
                            disabled={deleteDisabled}
                            title={
                              deleteDisabled
                                ? 'Нельзя удалить: есть привязанные субаккаунты'
                                : undefined
                            }
                            className="px-3 py-1.5 text-sm text-red-700 outline-none cursor-pointer hover:bg-red-50 data-[highlighted]:bg-red-50 data-[disabled]:text-slate-400 data-[disabled]:cursor-not-allowed data-[disabled]:hover:bg-transparent"
                            onSelect={() => {
                              if (!deleteDisabled) handleDelete(r.id);
                            }}
                          >
                            Удалить
                          </DropdownMenu.Item>
                        </DropdownMenu.Content>
                      </DropdownMenu.Portal>
                    </DropdownMenu.Root>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      )}
      <CreateTemplateDialog open={createOpen} onOpenChange={setCreateOpen} />
      <BindTemplateDialog
        open={bindTarget !== null}
        onOpenChange={(o) => {
          if (!o) setBindTarget(null);
        }}
        template={bindTarget}
        onSuccess={() => {
          refetch();
        }}
      />
      <DuplicateTemplateDialog
        open={duplicateTarget !== null}
        onOpenChange={(o) => {
          if (!o) setDuplicateTarget(null);
        }}
        sourceTemplate={duplicateTarget}
        onSuccess={() => {
          refetch();
        }}
      />
    </div>
  );
}
