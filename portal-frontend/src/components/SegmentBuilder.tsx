import { useState, useCallback } from 'react';
import { Button } from './ui/Button';

export interface SegmentCondition {
  field: string;
  operator: string;
  value: string;
}

export interface SegmentGroup {
  logic: 'AND' | 'OR';
  conditions: SegmentCondition[];
  groups: SegmentGroup[];
}

interface SegmentBuilderProps {
  fields: { name: string; label: string; type: string }[];
  value?: SegmentGroup;
  onChange: (group: SegmentGroup) => void;
  maxDepth?: number;
}

const OPERATORS: Record<string, { value: string; label: string }[]> = {
  string: [
    { value: 'eq', label: 'равно' },
    { value: 'neq', label: 'не равно' },
    { value: 'contains', label: 'содержит' },
    { value: 'starts_with', label: 'начинается с' },
    { value: 'ends_with', label: 'заканчивается на' },
    { value: 'is_empty', label: 'пусто' },
    { value: 'is_not_empty', label: 'не пусто' },
  ],
  number: [
    { value: 'eq', label: '=' },
    { value: 'neq', label: '!=' },
    { value: 'gt', label: '>' },
    { value: 'gte', label: '>=' },
    { value: 'lt', label: '<' },
    { value: 'lte', label: '<=' },
  ],
  date: [
    { value: 'eq', label: 'равно' },
    { value: 'before', label: 'до' },
    { value: 'after', label: 'после' },
  ],
  boolean: [
    { value: 'eq', label: 'равно' },
  ],
};

function defaultGroup(): SegmentGroup {
  return { logic: 'AND', conditions: [], groups: [] };
}

export function SegmentBuilder({
  fields,
  value,
  onChange,
  maxDepth = 3,
}: SegmentBuilderProps) {
  const [group, setGroup] = useState<SegmentGroup>(value ?? defaultGroup());

  const notify = useCallback(
    (g: SegmentGroup) => {
      setGroup(g);
      onChange(g);
    },
    [onChange],
  );

  return (
    <GroupEditor
      group={group}
      fields={fields}
      depth={0}
      maxDepth={maxDepth}
      onChange={notify}
      onRemove={null}
    />
  );
}

interface GroupEditorProps {
  group: SegmentGroup;
  fields: { name: string; label: string; type: string }[];
  depth: number;
  maxDepth: number;
  onChange: (group: SegmentGroup) => void;
  onRemove: (() => void) | null;
}

function GroupEditor({
  group,
  fields,
  depth,
  maxDepth,
  onChange,
  onRemove,
}: GroupEditorProps) {
  function toggleLogic() {
    onChange({ ...group, logic: group.logic === 'AND' ? 'OR' : 'AND' });
  }

  function addCondition() {
    const firstField = fields[0];
    const fieldType = firstField?.type ?? 'string';
    const ops = OPERATORS[fieldType] ?? OPERATORS['string'];
    onChange({
      ...group,
      conditions: [
        ...group.conditions,
        {
          field: firstField?.name ?? '',
          operator: ops[0]?.value ?? 'eq',
          value: '',
        },
      ],
    });
  }

  function updateCondition(idx: number, cond: SegmentCondition) {
    onChange({
      ...group,
      conditions: group.conditions.map((c, i) => (i === idx ? cond : c)),
    });
  }

  function removeCondition(idx: number) {
    onChange({
      ...group,
      conditions: group.conditions.filter((_, i) => i !== idx),
    });
  }

  function addNestedGroup() {
    if (depth >= maxDepth - 1) return;
    onChange({
      ...group,
      groups: [...group.groups, defaultGroup()],
    });
  }

  function updateNestedGroup(idx: number, g: SegmentGroup) {
    onChange({
      ...group,
      groups: group.groups.map((ng, i) => (i === idx ? g : ng)),
    });
  }

  function removeNestedGroup(idx: number) {
    onChange({
      ...group,
      groups: group.groups.filter((_, i) => i !== idx),
    });
  }

  return (
    <div
      className={`border rounded-lg p-3 ${depth === 0 ? 'border-gray-300 bg-white' : 'border-gray-200 bg-gray-50'}`}
    >
      <div className="flex items-center gap-2 mb-3">
        <button
          type="button"
          onClick={toggleLogic}
          className={`px-3 py-1 rounded-full text-xs font-semibold ${
            group.logic === 'AND'
              ? 'bg-blue-100 text-blue-700'
              : 'bg-orange-100 text-orange-700'
          }`}
        >
          {group.logic === 'AND' ? 'И (AND)' : 'ИЛИ (OR)'}
        </button>
        <span className="text-xs text-gray-400">
          Нажмите для переключения
        </span>
        {onRemove && (
          <button
            type="button"
            onClick={onRemove}
            className="ml-auto text-red-400 hover:text-red-600 text-sm"
          >
            Удалить группу
          </button>
        )}
      </div>

      {/* Conditions */}
      <div className="space-y-2">
        {group.conditions.map((cond, idx) => {
          const fieldDef = fields.find((f) => f.name === cond.field);
          const fieldType = fieldDef?.type ?? 'string';
          const ops = OPERATORS[fieldType] ?? OPERATORS['string'];

          return (
            <div key={idx} className="flex items-center gap-2">
              <select
                value={cond.field}
                onChange={(e) => {
                  const newField = fields.find(
                    (f) => f.name === e.target.value,
                  );
                  const newType = newField?.type ?? 'string';
                  const newOps = OPERATORS[newType] ?? OPERATORS['string'];
                  updateCondition(idx, {
                    ...cond,
                    field: e.target.value,
                    operator: newOps[0]?.value ?? 'eq',
                  });
                }}
                className="border border-gray-300 rounded px-2 py-1.5 text-sm flex-1 min-w-0 focus:outline-none focus:ring-2 focus:ring-blue-500"
              >
                {fields.map((f) => (
                  <option key={f.name} value={f.name}>
                    {f.label}
                  </option>
                ))}
              </select>

              <select
                value={cond.operator}
                onChange={(e) =>
                  updateCondition(idx, { ...cond, operator: e.target.value })
                }
                className="border border-gray-300 rounded px-2 py-1.5 text-sm w-36 focus:outline-none focus:ring-2 focus:ring-blue-500"
              >
                {ops.map((op) => (
                  <option key={op.value} value={op.value}>
                    {op.label}
                  </option>
                ))}
              </select>

              {cond.operator !== 'is_empty' &&
                cond.operator !== 'is_not_empty' && (
                  <input
                    type={fieldType === 'number' ? 'number' : 'text'}
                    value={cond.value}
                    onChange={(e) =>
                      updateCondition(idx, { ...cond, value: e.target.value })
                    }
                    placeholder="Значение"
                    className="border border-gray-300 rounded px-2 py-1.5 text-sm flex-1 min-w-0 focus:outline-none focus:ring-2 focus:ring-blue-500"
                  />
                )}

              <button
                type="button"
                onClick={() => removeCondition(idx)}
                className="text-red-400 hover:text-red-600 px-1"
                aria-label="Удалить условие"
              >
                X
              </button>
            </div>
          );
        })}
      </div>

      {/* Nested groups */}
      {group.groups.length > 0 && (
        <div className="mt-3 space-y-3">
          {group.groups.map((ng, idx) => (
            <GroupEditor
              key={idx}
              group={ng}
              fields={fields}
              depth={depth + 1}
              maxDepth={maxDepth}
              onChange={(g) => updateNestedGroup(idx, g)}
              onRemove={() => removeNestedGroup(idx)}
            />
          ))}
        </div>
      )}

      {/* Actions */}
      <div className="mt-3 flex gap-2">
        <Button variant="ghost" size="sm" onClick={addCondition}>
          + Условие
        </Button>
        {depth < maxDepth - 1 && (
          <Button variant="ghost" size="sm" onClick={addNestedGroup}>
            + Вложенная группа
          </Button>
        )}
      </div>
    </div>
  );
}
