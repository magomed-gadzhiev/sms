import { Select } from '../../../components/ui/Select';
import { SearchableSelect } from '../../../components/ui/SearchableSelect';
import type { ConditionGroupJSON, ConditionJSON } from '../types';
import { LOGIC_OP_LABELS, CONDITION_TYPE_LABELS } from '../types';
import type { OperatorInfo } from '../../../api/client';

const TRAFFIC_TYPES = [
  { value: 'authorization', label: 'Авторизация' },
  { value: 'transactional', label: 'Транзакционный' },
  { value: 'service', label: 'Сервисный' },
];

interface ConditionEditorProps {
  groups: ConditionGroupJSON[];
  onChange: (groups: ConditionGroupJSON[]) => void;
  operators?: OperatorInfo[];
}

const LOGIC_OPS: ConditionGroupJSON['logic_op'][] = ['IF', 'AND', 'AND_NOT', 'OR', 'OR_NOT'];
const CONDITION_TYPES: ConditionJSON['type'][] = ['operator', 'country', 'traffic_type', 'paid_name', 'regex'];

export function ConditionEditor({ groups, onChange, operators = [] }: ConditionEditorProps) {
  function updateGroup(index: number, updated: ConditionGroupJSON) {
    const next = [...groups];
    next[index] = updated;
    onChange(next);
  }

  function removeGroup(index: number) {
    onChange(groups.filter((_, i) => i !== index));
  }

  function addGroup() {
    onChange([
      ...groups,
      { logic_op: 'AND', conditions: [{ type: 'operator', value: '' }] },
    ]);
  }

  function addCondition(groupIndex: number) {
    const group = groups[groupIndex];
    updateGroup(groupIndex, {
      ...group,
      conditions: [...group.conditions, { type: 'operator', value: '' }],
    });
  }

  function updateCondition(groupIndex: number, condIndex: number, cond: ConditionJSON) {
    const group = groups[groupIndex];
    const conditions = [...group.conditions];
    conditions[condIndex] = cond;
    updateGroup(groupIndex, { ...group, conditions });
  }

  function removeCondition(groupIndex: number, condIndex: number) {
    const group = groups[groupIndex];
    updateGroup(groupIndex, {
      ...group,
      conditions: group.conditions.filter((_, i) => i !== condIndex),
    });
  }

  return (
    <div className="flex flex-col gap-4">
      {groups.map((group, gi) => (
        <div key={gi} className="border rounded-lg p-4 bg-gray-50">
          <div className="flex items-center justify-between mb-3">
            <div className="flex items-center gap-2">
              {gi === 0 ? (
                <span className="text-sm font-semibold text-gray-700">ЕСЛИ</span>
              ) : (
                <Select
                  value={group.logic_op}
                  onChange={(val) => updateGroup(gi, { ...group, logic_op: val as ConditionGroupJSON['logic_op'] })}
                  options={LOGIC_OPS.filter((op) => op !== 'IF').map((op) => ({
                    value: op,
                    label: LOGIC_OP_LABELS[op],
                  }))}
                />
              )}
            </div>
            {gi > 0 && (
              <button
                type="button"
                onClick={() => removeGroup(gi)}
                className="text-xs text-red-600 hover:text-red-800"
              >
                Удалить группу
              </button>
            )}
          </div>

          <div className="flex flex-col gap-2">
            {group.conditions.map((cond, ci) => (
              <div key={ci} className="flex items-center gap-2">
                <Select
                  value={cond.type}
                  onChange={(val) => updateCondition(gi, ci, { type: val as ConditionJSON['type'], value: '' })}
                  options={CONDITION_TYPES.map((t) => ({ value: t, label: CONDITION_TYPE_LABELS[t] }))}
                />
                {cond.type === 'operator' ? (
                  <SearchableSelect
                    className="flex-1"
                    value={cond.value}
                    onChange={(val) => updateCondition(gi, ci, { ...cond, value: val })}
                    placeholder="Выберите оператора"
                    options={operators.map((o) => ({ value: o.id, label: o.name }))}
                  />
                ) : cond.type === 'traffic_type' ? (
                  <Select
                    className="flex-1"
                    value={cond.value}
                    onChange={(val) => updateCondition(gi, ci, { ...cond, value: val })}
                    placeholder="Выберите тип трафика"
                    options={TRAFFIC_TYPES}
                  />
                ) : (
                  <input
                    type="text"
                    value={cond.value}
                    onChange={(e) => updateCondition(gi, ci, { ...cond, value: e.target.value })}
                    placeholder="Значение"
                    className="flex-1 rounded border border-gray-300 px-3 py-2 text-sm focus:outline-none focus-visible:ring-2 focus-visible:ring-primary/50"
                  />
                )}
                <button
                  type="button"
                  onClick={() => removeCondition(gi, ci)}
                  className="text-gray-400 hover:text-red-600 px-1"
                  aria-label="Удалить условие"
                >
                  ✕
                </button>
              </div>
            ))}
          </div>

          <button
            type="button"
            onClick={() => addCondition(gi)}
            className="mt-2 text-xs text-primary hover:underline"
          >
            + Условие
          </button>
        </div>
      ))}

      <button
        type="button"
        onClick={addGroup}
        className="text-sm text-primary hover:underline self-start"
      >
        + Добавить группу условий
      </button>
    </div>
  );
}
