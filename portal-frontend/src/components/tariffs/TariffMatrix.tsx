/**
 * TariffMatrix — shared read/edit matrix for network tariffs.
 *
 * Responsibilities:
 *  - render operators × tiers matrix from TariffEditorData (read mode)
 *  - on demand, switch to edit mode with per-cell drafts, validation, keyboard nav
 *  - emit a TariffBulkSaveBody diff through onSaveBatch; stay in edit mode on errors
 *  - surface inheritance markers (template vs override vs unset) when showInheritance=true
 *
 * Non-goals (consumer's concern):
 *  - fetching data, switching periods, reloading after save success (parent re-feeds `data` via props)
 *  - routing, permissions, page-level beforeunload (parent wires `onUnsavedChange`)
 */

import {
  useCallback,
  useEffect,
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
  TariffMatrixProps,
  TariffMatrixSaveError,
  TariffBulkSaveBody,
} from './TariffMatrix.types';
import { formatPrice, currencySymbol } from './formatPrice';
import { formatQuantity } from './formatQuantity';

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

export function TariffMatrix(props: TariffMatrixProps) {
  const { data, scope, editable, showInheritance, currency, onSaveBatch, onUnsavedChange } =
    props;

  const [editMode, setEditMode] = useState(false);
  const [drafts, setDrafts] = useState<Map<string, DraftCell>>(new Map());
  const [errors, setErrors] = useState<Map<string, string>>(new Map());
  const [saving, setSaving] = useState(false);
  const [activeKey, setActiveKey] = useState<string | null>(null);
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

  // when parent swaps `data` (e.g. after successful save or period change), reset local state.
  useEffect(() => {
    setDrafts(new Map());
    setErrors(new Map());
    setActiveKey(null);
  }, [data]);

  const exitEditMode = useCallback(() => {
    setEditMode(false);
    setDrafts(new Map());
    setErrors(new Map());
    setActiveKey(null);
  }, []);

  const enterEditMode = useCallback(() => {
    setEditMode(true);
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
    try {
      const res = await onSaveBatch(buildBatch());
      if (res.ok) {
        // parent re-feeds fresh `data`; effect above will clear drafts.
        setEditMode(false);
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
    }
  }, [onSaveBatch, saving, drafts, buildBatch]);

  const handleCancel = useCallback(() => {
    exitEditMode();
  }, [exitEditMode]);

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
          title="Переопределение"
          aria-label={ariaLabel}
        >
          <span aria-hidden="true" className="mr-1 text-amber-600">
            ●
          </span>
          {formatted}
        </span>
      );
    }

    // source === 'template' in override-scope view → inherited display
    if (scope.kind === 'override') {
      return (
        <span
          className="block text-right italic text-slate-500 tabular-nums"
          title="Из шаблона"
          aria-label={`${formatted}, унаследовано из шаблона`}
        >
          {formatted}
        </span>
      );
    }

    // template-scope view of a template cell → plain
    return <span className="block text-right tabular-nums">{formatted}</span>;
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

  const showEditButton = editable && !editMode && scope.kind !== 'client';
  const showEditControls = editable && editMode && scope.kind !== 'client';

  return (
    <div className="tariff-matrix">
      {showEditButton ? (
        <div className="mb-3 flex justify-end">
          <Button variant="secondary" size="sm" onClick={enterEditMode}>
            Режим правки
          </Button>
        </div>
      ) : null}

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
                  {formatQuantity(tier.from_quantity)}
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
                  return (
                    <td
                      key={tier.id}
                      className={`border-b border-slate-100 px-2 py-1 align-middle ${
                        isActive ? 'bg-primary/5' : ''
                      }`}
                    >
                      {showEditControls
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

      {showEditControls ? (
        <div
          role="status"
          aria-live="polite"
          className="sticky bottom-0 mt-3 flex items-center justify-between border-t border-slate-200 bg-white/95 px-3 py-2 backdrop-blur"
        >
          <span className="text-sm text-slate-700">
            Несохранённых изменений: <span className="font-semibold">{dirtyCount}</span>
          </span>
          <div className="flex items-center gap-2">
            <Button variant="secondary" size="sm" onClick={handleCancel} disabled={saving}>
              Отмена
            </Button>
            <Button
              variant="primary"
              size="sm"
              onClick={handleSave}
              disabled={saving || dirtyCount === 0}
            >
              {saving ? 'Сохранение…' : 'Сохранить'}
            </Button>
          </div>
        </div>
      ) : null}
    </div>
  );
}

export default TariffMatrix;
