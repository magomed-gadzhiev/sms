import { } from 'react';

interface Condition {
  field: string;
  op: string;
  value: string;
}

interface RuleGroup {
  operator: 'AND' | 'OR';
  conditions: Condition[];
}

interface SegmentBuilderProps {
  rules: RuleGroup;
  onChange: (rules: RuleGroup) => void;
}

const OPERATORS = [
  { value: 'eq', label: '=' },
  { value: 'neq', label: '!=' },
  { value: 'gt', label: '>' },
  { value: 'gte', label: '>=' },
  { value: 'lt', label: '<' },
  { value: 'lte', label: '<=' },
  { value: 'contains', label: 'содержит' },
  { value: 'in', label: 'в списке' },
  { value: 'is_empty', label: 'пусто' },
  { value: 'is_not_empty', label: 'не пусто' },
  { value: 'older_than_days', label: 'старше (дней)' },
  { value: 'newer_than_days', label: 'новее (дней)' },
];

export function SegmentBuilder({ rules, onChange }: SegmentBuilderProps) {
  const addCondition = () => {
    onChange({
      ...rules,
      conditions: [...rules.conditions, { field: '', op: 'eq', value: '' }],
    });
  };

  const updateCondition = (index: number, updates: Partial<Condition>) => {
    const newConditions = [...rules.conditions];
    newConditions[index] = { ...newConditions[index], ...updates };
    onChange({ ...rules, conditions: newConditions });
  };

  const removeCondition = (index: number) => {
    onChange({
      ...rules,
      conditions: rules.conditions.filter((_, i) => i !== index),
    });
  };

  return (
    <div className="space-y-3">
      <div className="flex items-center gap-2">
        <span className="text-sm text-gray-600">Совпадение:</span>
        <select
          value={rules.operator}
          onChange={(e) => onChange({ ...rules, operator: e.target.value as 'AND' | 'OR' })}
          className="px-2 py-1 border border-gray-300 rounded text-sm"
        >
          <option value="AND">Все условия (AND)</option>
          <option value="OR">Любое условие (OR)</option>
        </select>
      </div>

      {rules.conditions.map((cond, i) => (
        <div key={i} className="flex items-center gap-2 p-2 bg-gray-50 rounded">
          <input
            value={cond.field}
            onChange={(e) => updateCondition(i, { field: e.target.value })}
            placeholder="attributes.city"
            className="flex-1 px-2 py-1 border border-gray-300 rounded text-sm"
          />
          <select
            value={cond.op}
            onChange={(e) => updateCondition(i, { op: e.target.value })}
            className="px-2 py-1 border border-gray-300 rounded text-sm"
          >
            {OPERATORS.map((op) => (
              <option key={op.value} value={op.value}>{op.label}</option>
            ))}
          </select>
          {!['is_empty', 'is_not_empty'].includes(cond.op) && (
            <input
              value={cond.value}
              onChange={(e) => updateCondition(i, { value: e.target.value })}
              placeholder="значение"
              className="flex-1 px-2 py-1 border border-gray-300 rounded text-sm"
            />
          )}
          <button onClick={() => removeCondition(i)} className="text-red-500 hover:text-red-700 text-sm px-1">
            x
          </button>
        </div>
      ))}

      <button onClick={addCondition} className="text-sm text-primary hover:underline">
        + Добавить условие
      </button>
    </div>
  );
}
