/**
 * TariffMatrix — shared read/edit matrix for network tariffs.
 *
 * Responsibilities:
 *  - render operators × tiers matrix from TariffEditorData (read mode)
 *  - when editable=true and scope.kind!='client', cells are always editable
 *    (no «Режим правки» toggle — parent owns the save/cancel surface)
 *  - emit a TariffBulkSaveBody diff through onSaveBatch; stay in edit state on errors
 *  - surface inheritance markers (template vs override vs unset) when showInheritance=true
 *  - expose TariffMatrixHandle via forwardRef for parent-driven save/reset
 *
 * Non-goals (consumer's concern):
 *  - fetching data, switching periods, reloading after save success (parent re-feeds `data` via props)
 *  - routing, permissions, page-level beforeunload (parent wires `onUnsavedChange`)
 *  - save/cancel UI controls (parent renders EditorSaveBar and calls ref.save()/ref.reset())
 */

import {
  forwardRef,
  useCallback,
  useEffect,
  useImperativeHandle,
  useMemo,
  useRef,
  useState,
  type KeyboardEvent,
  type ChangeEvent,
  type MouseEvent,
} from 'react';
import { Button } from '../ui/Button';
import type {
  TariffEditorCell,
  TariffMatrixHandle,
  TariffMatrixProps,
  TariffMatrixSaveError,
  TariffBulkSaveBody,
} from './TariffMatrix.types';
import { formatPrice, currencySymbol } from './formatPrice';
import { formatQuantity } from './formatQuantity';
import { useMediaQuery } from '../../hooks/useMediaQuery';
import { MobileOperatorCard } from './MobileOperatorCard';
import { MobileEditDrawer } from './MobileEditDrawer';

type DraftSource = 'template' | 'override' | 'unset';

interface DraftCell {
  price: number | null;
  /** raw string the user typed (lets us keep `''` distinct from `0`) */
  raw: string;
  originalSource: DraftSource;
  originalPrice: number | null;
  dirty: boolean;
  invalid: boolean;
}

function cellKey(operatorId: string, tierId: string): string {
  return `${operatorId}:${tierId}`;
}

function parsePrice(raw: string): { value: number | null; valid: boolean } {
  const trimmed = raw.trim();
  if (trimmed === '') return { value: null, valid: true }; // empty means "clear"
  const normalised = trimmed.replace(',', '.');
  const n = Number(normalised);
  if (!Number.isFinite(n)) return { value: null, valid: false };
  if (n < 0 || n > 999) return { value: n, valid: false };
  return { value: n, valid: true };
}

function operatorInitial(name: string): string {
  const c = name.trim().charAt(0);
  return c ? c.toUpperCase() : '?';
}

export const TariffMatrix = forwardRef<TariffMatrixHandle, TariffMatrixProps>(
  function TariffMatrix(props, ref) {
    const {
      data,
      scope,
      editable,
      showInheritance,
      currency,
      onSaveBatch,
      onUnsavedChange,
      onDirtyCountChange,
      onSavingChange,
    } = props;

    const [drafts, setDrafts] = useState<Map<string, DraftCell>>(new Map());
    const [errors, setErrors] = useState<Map<string, string>>(new Map());
    const [saving, setSaving] = useState(false);
    const [activeKey, setActiveKey] = useState<string | null>(null);
    const [newTierRaw, setNewTierRaw] = useState('');
    const [tierOpError, setTierOpError] = useState<string | null>(null);
    const inputRefs = useRef<Map<string, HTMLInputElement | null>>(new Map());

    const cellIndex = useMemo(() => {
      const m = new Map<string, TariffEditorCell>();
      for (const c of data.cells) m.set(cellKey(c.operator_id, c.tier_id), c);
      return m;
    }, [data.cells]);

    const sortedTiers = useMemo(
      () => [...data.tiers].sort((a, b) => a.from_quantity - b.from_quantity),
      [data.tiers],
    );

    const dirtyCount = useMemo(() => {
      let n = 0;
      drafts.forEach((d) => {
        if (d.dirty) n += 1;
      });
      return n;
    }, [drafts]);

    const hasChanges = dirtyCount > 0;

    useEffect(() => {
      onUnsavedChange?.(hasChanges);
    }, [hasChanges, onUnsavedChange]);

    useEffect(() => {
      onDirtyCountChange?.(dirtyCount);
    }, [dirtyCount, onDirtyCountChange]);

    // when parent swaps to a DIFFERENT plan/period, reset local state. We key on
    // identity markers instead of `data` itself because the parent may re-create
    // the `data` object on every render (e.g. via spread) — keying on `data`
    // would nuke the user's in-flight edits after the first keystroke triggers
    // onUnsavedChange → parent re-render → new `data` reference.
    useEffect(() => {
      setDrafts(new Map());
      setErrors(new Map());
      setActiveKey(null);
    }, [data.plan.id, data.active_period_id]);

    const resetDrafts = useCallback(() => {
      setDrafts(new Map());
      setErrors(new Map());
      setActiveKey(null);
    }, []);

    const ensureDraft = useCallback(
      (opId: string, tierId: string): DraftCell => {
        const key = cellKey(opId, tierId);
        const existing = drafts.get(key);
        if (existing) return existing;
        const cell = cellIndex.get(key);
        const source: DraftSource = (cell?.source ?? 'unset') as DraftSource;
        const originalPrice =
          source === 'override'
            ? cell?.price_override ?? null
            : source === 'template'
              ? cell?.price_template ?? null
              : null;
        const effective = cell?.effective ?? originalPrice;
        return {
          price: effective,
          raw: effective === null ? '' : String(effective),
          originalSource: source,
          originalPrice,
          dirty: false,
          invalid: false,
        };
      },
      [cellIndex, drafts],
    );

    const setDraft = useCallback(
      (opId: string, tierId: string, raw: string) => {
        const key = cellKey(opId, tierId);
        setDrafts((prev) => {
          const next = new Map(prev);
          const base = next.get(key) ?? ensureDraft(opId, tierId);
          const { value, valid } = parsePrice(raw);
          const dirty = (() => {
            // Clearing a cell that was unset or template-inherited: not an override until blur.
            if (value === base.originalPrice && base.originalSource !== 'unset') {
              // Same value as inherited/override → no change from its origin.
              // For inherited (template in override scope), do NOT create override.
              return false;
            }
            if (value === null && base.originalSource === 'unset') return false;
            return true;
          })();
          next.set(key, {
            ...base,
            raw,
            price: value,
            dirty,
            invalid: !valid,
          });
          return next;
        });
        // clear any persisted save-error for this cell; user is typing a fix.
        setErrors((prev) => {
          if (!prev.has(key)) return prev;
          const next = new Map(prev);
          next.delete(key);
          return next;
        });
      },
      [ensureDraft],
    );

    const revertCell = useCallback((opId: string, tierId: string) => {
      const key = cellKey(opId, tierId);
      setDrafts((prev) => {
        if (!prev.has(key)) return prev;
        const next = new Map(prev);
        next.delete(key);
        return next;
      });
      setErrors((prev) => {
        if (!prev.has(key)) return prev;
        const next = new Map(prev);
        next.delete(key);
        return next;
      });
    }, []);

    const focusCell = useCallback((key: string | null) => {
      if (!key) return;
      const el = inputRefs.current.get(key);
      if (el) {
        el.focus();
        el.select();
      }
    }, []);

    const handleInputKeyDown = useCallback(
      (
        e: KeyboardEvent<HTMLInputElement>,
        opIdx: number,
        tierIdx: number,
        opId: string,
        tierId: string,
      ) => {
        const operators = data.operators;
        const tiers = sortedTiers;

        if (e.key === 'Escape') {
          e.preventDefault();
          revertCell(opId, tierId);
          return;
        }
        if (e.key === 'Enter') {
          e.preventDefault();
          const nextOp = operators[opIdx + 1];
          if (nextOp) {
            const k = cellKey(nextOp.id, tierId);
            setActiveKey(k);
            focusCell(k);
          } else {
            e.currentTarget.blur();
          }
          return;
        }
        if (e.key === 'Tab') {
          // let the browser handle focus, but advance through our active tracker.
          const dir = e.shiftKey ? -1 : 1;
          let nextTierIdx = tierIdx + dir;
          let nextOpIdx = opIdx;
          if (nextTierIdx >= tiers.length) {
            nextTierIdx = 0;
            nextOpIdx += 1;
          } else if (nextTierIdx < 0) {
            nextTierIdx = tiers.length - 1;
            nextOpIdx -= 1;
          }
          const nextOp = operators[nextOpIdx];
          const nextTier = tiers[nextTierIdx];
          if (nextOp && nextTier) {
            setActiveKey(cellKey(nextOp.id, nextTier.id));
          }
          // default Tab behaviour handles the actual focus.
        }
      },
      [data.operators, sortedTiers, revertCell, focusCell],
    );

    const buildBatch = useCallback((): TariffBulkSaveBody => {
      const scopeKind: 'template' | 'override' =
        scope.kind === 'override' ? 'override' : 'template';
      const subAccountId = scope.kind === 'override' ? scope.subAccountId : undefined;
      const cellsUpsert: TariffBulkSaveBody['cells_upsert'] = [];
      const cellsDelete: TariffBulkSaveBody['cells_delete'] = [];
      drafts.forEach((d, key) => {
        if (!d.dirty || d.invalid) return;
        const [operator_id, tier_id] = key.split(':');
        if (d.price === null) {
          cellsDelete.push({
            operator_id,
            tier_id,
            scope: scopeKind,
            ...(subAccountId ? { sub_account_id: subAccountId } : {}),
          });
        } else {
          cellsUpsert.push({
            operator_id,
            tier_id,
            price: d.price,
            scope: scopeKind,
            ...(subAccountId ? { sub_account_id: subAccountId } : {}),
          });
        }
      });
      return {
        period_id: data.active_period_id,
        tiers_upsert: [], // matrix does not add/remove tiers; that's a separate flow.
        tiers_delete: [],
        cells_upsert: cellsUpsert,
        cells_delete: cellsDelete,
      };
    }, [drafts, scope, data.active_period_id]);

    const handleSave = useCallback(async () => {
      if (!onSaveBatch || saving) return;
      // any invalid draft blocks save.
      let invalid = false;
      drafts.forEach((d) => {
        if (d.invalid) invalid = true;
      });
      if (invalid) return;

      setSaving(true);
      onSavingChange?.(true);
      try {
        const res = await onSaveBatch(buildBatch());
        if (res.ok) {
          // parent re-feeds fresh `data`; effect above will clear drafts.
          setDrafts(new Map());
          setErrors(new Map());
        } else {
          const map = new Map<string, string>();
          (res.errors ?? []).forEach((err: TariffMatrixSaveError) => {
            map.set(cellKey(err.operator_id, err.tier_id), err.reason);
          });
          setErrors(map);
        }
      } finally {
        setSaving(false);
        onSavingChange?.(false);
      }
    }, [onSaveBatch, saving, drafts, buildBatch, onSavingChange]);

    // Expose imperative handle so parent (EditorSaveBar's host) can trigger save/reset.
    useImperativeHandle(
      ref,
      () => ({
        save: handleSave,
        reset: resetDrafts,
        hasUnsavedChanges: () => dirtyCount > 0,
      }),
      [handleSave, resetDrafts, dirtyCount],
    );

    // Tier ops — separate from the cell-drafts flow. Each click sends an
    // isolated batch (tiers_upsert/delete only, no cells). The parent refetches
    // on success, which picks up the new tier set without wiping cell drafts
    // (useEffect reset keys on [plan.id, active_period_id], not `data`).
    const submitTierAdd = useCallback(async () => {
      if (!onSaveBatch || saving) return;
      const raw = newTierRaw.trim().replace(',', '.');
      if (raw === '') {
        setTierOpError('Введите объём (сегментов)');
        return;
      }
      const q = Number(raw);
      if (!Number.isFinite(q) || q < 0 || !Number.isInteger(q)) {
        setTierOpError('Объём должен быть неотрицательным целым числом');
        return;
      }
      if (sortedTiers.some((t) => t.from_quantity === q)) {
        setTierOpError('Ступень с таким объёмом уже существует');
        return;
      }
      setTierOpError(null);
      setSaving(true);
      onSavingChange?.(true);
      try {
        const res = await onSaveBatch({
          period_id: data.active_period_id,
          tiers_upsert: [{ id: null, from_quantity: q, price_per_segment: 0 }],
          tiers_delete: [],
          cells_upsert: [],
          cells_delete: [],
        });
        if (res.ok) {
          setNewTierRaw('');
        } else {
          setTierOpError('Не удалось добавить ступень');
        }
      } finally {
        setSaving(false);
        onSavingChange?.(false);
      }
    }, [onSaveBatch, saving, newTierRaw, sortedTiers, data.active_period_id, onSavingChange]);

    const submitTierDelete = useCallback(
      async (tierId: string, label: string) => {
        if (!onSaveBatch || saving) return;
        if (!window.confirm(`Удалить ступень «${label}»? Цены по ней будут потеряны.`)) {
          return;
        }
        setTierOpError(null);
        setSaving(true);
        onSavingChange?.(true);
        try {
          const res = await onSaveBatch({
            period_id: data.active_period_id,
            tiers_upsert: [],
            tiers_delete: [tierId],
            cells_upsert: [],
            cells_delete: [],
          });
          if (!res.ok) {
            setTierOpError('Не удалось удалить ступень');
          }
        } finally {
          setSaving(false);
          onSavingChange?.(false);
        }
      },
      [onSaveBatch, saving, data.active_period_id, onSavingChange],
    );

    const renderReadCell = (cell: TariffEditorCell | undefined) => {
      const source = cell?.source ?? 'unset';
      const effective = cell?.effective ?? null;

      if (source === 'unset' || effective === null) {
        return (
          <span className="block text-center text-slate-400" aria-label="значение не задано">
            —
          </span>
        );
      }

      const formatted = formatPrice(effective, currency);

      if (!showInheritance) {
        return <span className="block text-right tabular-nums">{formatted}</span>;
      }

      if (source === 'override') {
        const base = cell?.price_template;
        const ariaLabel =
          base !== null && base !== undefined
            ? `${formatted}, переопределение поверх значения ${formatPrice(base, currency)} из шаблона`
            : `${formatted}, переопределение`;
        return (
          <span
            className="block text-right font-semibold tabular-nums"
            aria-label={ariaLabel}
          >
            {formatted}
          </span>
        );
      }

      // source === 'template' in override-scope view → inherited display
      if (scope.kind === 'override') {
        return (
          <span
            className="block text-right tabular-nums"
            aria-label={`${formatted}, унаследовано из шаблона`}
          >
            {formatted}
          </span>
        );
      }

      // template-scope view of a template cell → plain
      return <span className="block text-right tabular-nums">{formatted}</span>;
    };

    const cellTint = (cell: TariffEditorCell | undefined): { tintClass: string; tintTitle: string } => {
      if (!showInheritance || scope.kind !== 'override') {
        return { tintClass: '', tintTitle: '' };
      }
      const source = cell?.source ?? 'unset';
      if (source === 'override') {
        return {
          tintClass: 'bg-violet-50 dark:bg-violet-950/30 border-l-2 border-violet-400 dark:border-violet-700',
          tintTitle: 'Переопределено в этом субаккаунте',
        };
      }
      if (source === 'template') {
        const templatePrice = cell?.price_template;
        return {
          tintClass: 'bg-emerald-50 dark:bg-emerald-950/30 border-l-2 border-emerald-400 dark:border-emerald-700',
          tintTitle:
            templatePrice != null
              ? `Цена из шаблона (${templatePrice} ${currencySymbol(currency)})`
              : 'Цена из шаблона',
        };
      }
      // unset
      return {
        tintClass: '',
        tintTitle: 'Не задано — будет использован fallback',
      };
    };

    const renderEditCell = (
      cell: TariffEditorCell | undefined,
      opIdx: number,
      tierIdx: number,
      operatorId: string,
      operatorName: string,
      tierId: string,
      tierLabel: string,
    ) => {
      const key = cellKey(operatorId, tierId);
      const draft = drafts.get(key);
      const source: DraftSource = (cell?.source ?? 'unset') as DraftSource;
      const effective = cell?.effective ?? null;
      const displayValue =
        draft?.raw ?? (effective === null ? '' : String(effective));
      const saveError = errors.get(key);
      const invalid = draft?.invalid ?? false;
      const showRevert =
        draft?.dirty &&
        (source === 'override' ||
          (scope.kind === 'override' && source === 'template' && draft.price !== cell?.price_template));

      const ariaLabel = `${operatorName}, ${tierLabel}, цена в ${currencySymbol(currency)}`;

      const ringClass = saveError || invalid ? 'ring-2 ring-red-500' : 'focus:ring-2 focus:ring-primary/50';
      const bgClass =
        draft?.dirty && !invalid && !saveError
          ? 'bg-amber-50'
          : '';

      return (
        <div className="relative flex items-center gap-1">
          <input
            ref={(el) => {
              inputRefs.current.set(key, el);
            }}
            type="text"
            inputMode="decimal"
            value={displayValue}
            aria-label={ariaLabel}
            aria-invalid={invalid || Boolean(saveError)}
            title={saveError ?? (invalid ? 'Допустимо: число от 0 до 999' : undefined)}
            onFocus={() => setActiveKey(key)}
            onChange={(e: ChangeEvent<HTMLInputElement>) =>
              setDraft(operatorId, tierId, e.target.value)
            }
            onKeyDown={(e) => handleInputKeyDown(e, opIdx, tierIdx, operatorId, tierId)}
            className={`w-20 rounded border border-gray-300 bg-white px-2 py-1 text-right text-sm tabular-nums outline-none ${ringClass} ${bgClass}`}
          />
          {showRevert ? (
            <button
              type="button"
              onClick={(e: MouseEvent<HTMLButtonElement>) => {
                e.preventDefault();
                revertCell(operatorId, tierId);
              }}
              className="text-xs text-slate-500 hover:text-slate-800"
              title="Вернуть унаследованное значение"
              aria-label={`Вернуть унаследованное значение для ${operatorName}, ${tierLabel}`}
            >
              ×
            </button>
          ) : null}
        </div>
      );
    };

    // Always editable when editable=true and scope is not client.
    // renderEditCell is used directly; renderReadCell is used for editable=false consumers.
    const alwaysEdit = editable && scope.kind !== 'client';
    // Show tier-management controls only when in always-edit mode.
    const showTierControls = alwaysEdit;

    // Mobile layout: on viewports <md replace the table with operator cards + bottom drawer.
    // useMediaQuery must be called unconditionally (rules of hooks); branch on its value below.
    const isDesktop = useMediaQuery('(min-width: 768px)');
    const [drawerOpId, setDrawerOpId] = useState<string | null>(null);

    if (!isDesktop && alwaysEdit) {
      const drawerOp = drawerOpId
        ? data.operators.find((o) => o.id === drawerOpId) ?? null
        : null;

      return (
        <div className="tariff-matrix">
          {/* Tier-add control — copied from desktop footer; must remain accessible on mobile. */}
          {showTierControls ? (
            <div className="mb-3 flex flex-wrap items-center gap-2 rounded border border-dashed border-slate-200 bg-slate-50 px-3 py-2 text-sm">
              <span className="text-slate-700">Добавить ступень от объёма:</span>
              <input
                type="text"
                inputMode="numeric"
                value={newTierRaw}
                onChange={(e) => {
                  setNewTierRaw(e.target.value);
                  if (tierOpError) setTierOpError(null);
                }}
                placeholder="например, 1000"
                aria-label="Объём сегментов для новой ступени"
                className="w-32 rounded border border-slate-300 px-2 py-1 text-sm outline-none focus:ring-2 focus:ring-primary/50"
              />
              <Button
                variant="secondary"
                size="sm"
                onClick={submitTierAdd}
                disabled={saving || newTierRaw.trim() === ''}
              >
                + Добавить
              </Button>
              {tierOpError ? (
                <span role="alert" className="text-xs text-red-600">
                  {tierOpError}
                </span>
              ) : null}
            </div>
          ) : null}

          {/* Operator card list */}
          <div className="space-y-2">
            {data.operators.map((op) => {
              const cellsByTier = new Map<string, TariffEditorCell | undefined>();
              for (const t of sortedTiers) {
                cellsByTier.set(t.id, cellIndex.get(cellKey(op.id, t.id)));
              }
              return (
                <MobileOperatorCard
                  key={op.id}
                  operator={op}
                  tiers={sortedTiers}
                  cellsByTier={cellsByTier}
                  currency={currency}
                  showInheritance={!!showInheritance && scope.kind === 'override'}
                  onEdit={() => setDrawerOpId(op.id)}
                />
              );
            })}
            {data.operators.length === 0 ? (
              <p className="px-3 py-6 text-center text-slate-500">
                Нет операторов для выбранной связки фильтров.
              </p>
            ) : null}
          </div>

          {/* Bottom drawer — only when an operator is selected */}
          {drawerOp && (
            <MobileEditDrawer
              operator={drawerOp}
              tiers={sortedTiers}
              currency={currency}
              initialValues={(() => {
                const m = new Map<string, string>();
                for (const t of sortedTiers) {
                  const key = cellKey(drawerOp.id, t.id);
                  const draft = drafts.get(key);
                  const cell = cellIndex.get(key);
                  const v =
                    draft?.raw ?? (cell?.effective != null ? String(cell.effective) : '');
                  m.set(t.id, v);
                }
                return m;
              })()}
              onCancel={() => setDrawerOpId(null)}
              onApply={(entries) => {
                entries.forEach((e) => setDraft(drawerOp.id, e.tierId, e.raw));
                setDrawerOpId(null);
              }}
            />
          )}
        </div>
      );
    }

    return (
      <div className="tariff-matrix">
        <div className="overflow-x-auto">
          <table className="min-w-full border-collapse text-sm">
            <caption className="sr-only">
              Тарифная сетка: operators × tiers для {data.scope.kind === 'template' ? data.scope.template_name : data.scope.sub_account_name}
            </caption>
            <thead className="bg-slate-50">
              <tr>
                <th
                  scope="col"
                  className="sticky left-0 z-10 border-b border-r border-slate-200 bg-slate-50 px-3 py-2 text-left font-medium text-slate-700"
                >
                  Оператор
                </th>
                {sortedTiers.map((tier) => (
                  <th
                    key={tier.id}
                    scope="col"
                    className="border-b border-slate-200 px-3 py-2 text-right font-medium text-slate-700 tabular-nums"
                  >
                    <span className="inline-flex items-center gap-1">
                      {formatQuantity(tier.from_quantity)}
                      {showTierControls && sortedTiers.length > 1 ? (
                        <button
                          type="button"
                          onClick={() =>
                            submitTierDelete(tier.id, formatQuantity(tier.from_quantity))
                          }
                          disabled={saving}
                          title="Удалить ступень"
                          aria-label={`Удалить ступень ${formatQuantity(tier.from_quantity)}`}
                          className="ml-1 inline-flex h-4 w-4 items-center justify-center rounded text-xs text-slate-400 hover:bg-red-50 hover:text-red-600 disabled:opacity-50"
                        >
                          ×
                        </button>
                      ) : null}
                    </span>
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {data.operators.map((op, opIdx) => (
                <tr key={op.id} className="hover:bg-slate-50/60">
                  <th
                    scope="row"
                    className="sticky left-0 z-10 border-b border-r border-slate-200 bg-white px-3 py-2 text-left font-normal text-slate-800"
                  >
                    <span className="inline-flex items-center gap-2">
                      {op.icon ? (
                        <img src={op.icon} alt="" aria-hidden="true" className="h-5 w-5 rounded-full" />
                      ) : (
                        <span
                          aria-hidden="true"
                          className="inline-flex h-5 w-5 items-center justify-center rounded-full bg-slate-200 text-xs font-semibold text-slate-700"
                        >
                          {operatorInitial(op.name)}
                        </span>
                      )}
                      <span>{op.name}</span>
                    </span>
                  </th>
                  {sortedTiers.map((tier, tierIdx) => {
                    const cell = cellIndex.get(cellKey(op.id, tier.id));
                    const key = cellKey(op.id, tier.id);
                    const tierLabel = formatQuantity(tier.from_quantity);
                    const isActive = activeKey === key;
                    const { tintClass, tintTitle } = cellTint(cell);
                    return (
                      <td
                        key={tier.id}
                        className={`border-b border-slate-100 px-2 py-1 align-middle ${
                          isActive ? 'bg-primary/5' : tintClass
                        }`}
                        title={tintTitle || undefined}
                      >
                        {alwaysEdit
                          ? renderEditCell(cell, opIdx, tierIdx, op.id, op.name, tier.id, tierLabel)
                          : renderReadCell(cell)}
                      </td>
                    );
                  })}
                </tr>
              ))}
              {data.operators.length === 0 ? (
                <tr>
                  <td
                    colSpan={sortedTiers.length + 1}
                    className="px-3 py-6 text-center text-slate-500"
                  >
                    Нет операторов для выбранной связки фильтров.
                  </td>
                </tr>
              ) : null}
            </tbody>
          </table>
        </div>

        {showTierControls ? (
          <div className="mt-3 flex flex-wrap items-center gap-2 rounded border border-dashed border-slate-200 bg-slate-50 px-3 py-2 text-sm">
            <span className="text-slate-700">Добавить ступень от объёма:</span>
            <input
              type="text"
              inputMode="numeric"
              value={newTierRaw}
              onChange={(e) => {
                setNewTierRaw(e.target.value);
                if (tierOpError) setTierOpError(null);
              }}
              placeholder="например, 1000"
              aria-label="Объём сегментов для новой ступени"
              className="w-32 rounded border border-slate-300 px-2 py-1 text-sm outline-none focus:ring-2 focus:ring-primary/50"
            />
            <Button
              variant="secondary"
              size="sm"
              onClick={submitTierAdd}
              disabled={saving || newTierRaw.trim() === ''}
            >
              + Добавить
            </Button>
            {tierOpError ? (
              <span role="alert" className="text-xs text-red-600">
                {tierOpError}
              </span>
            ) : null}
          </div>
        ) : null}
      </div>
    );
  },
);

export default TariffMatrix;
