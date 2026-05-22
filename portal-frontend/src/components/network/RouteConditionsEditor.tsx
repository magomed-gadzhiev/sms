import type { RouteCondition, RouteConditionGroup } from '../../api/client';

const TYPES: Array<RouteCondition['type']> = ['operator', 'country', 'traffic_type', 'paid_name', 'regex'];
const LOGIC_OPS: Array<RouteConditionGroup['logic_op']> = ['IF', 'AND', 'AND_NOT', 'OR', 'OR_NOT'];

interface Props {
  groups: RouteConditionGroup[];
  onChange: (groups: RouteConditionGroup[]) => void;
}

export function RouteConditionsEditor({ groups, onChange }: Props) {
  const addGroup = (): void => {
    onChange([...groups, { logic_op: 'IF', conditions: [{ type: 'country', value: '' }] }]);
  };
  const removeGroup = (idx: number): void => {
    onChange(groups.filter((_, i) => i !== idx));
  };
  const updateGroup = (idx: number, patch: Partial<RouteConditionGroup>): void => {
    onChange(groups.map((g, i) => (i === idx ? { ...g, ...patch } : g)));
  };
  const addCondition = (gIdx: number): void => {
    updateGroup(gIdx, { conditions: [...groups[gIdx].conditions, { type: 'country', value: '' }] });
  };
  const removeCondition = (gIdx: number, cIdx: number): void => {
    updateGroup(gIdx, { conditions: groups[gIdx].conditions.filter((_, i) => i !== cIdx) });
  };
  const updateCondition = (gIdx: number, cIdx: number, patch: Partial<RouteCondition>): void => {
    const next = [...groups[gIdx].conditions];
    next[cIdx] = { ...next[cIdx], ...patch };
    updateGroup(gIdx, { conditions: next });
  };

  return (
    <div className="space-y-3">
      <div className="text-sm font-medium text-gray-700">Условия</div>
      {groups.length === 0 && (
        <div className="text-xs text-gray-400">Без условий — правило применяется ко всем сообщениям</div>
      )}
      {groups.map((g, gIdx) => (
        <div key={gIdx} className="border border-gray-200 rounded p-3">
          <div className="flex items-center gap-2 mb-2">
            <select
              value={g.logic_op}
              onChange={(e) => updateGroup(gIdx, { logic_op: e.target.value as RouteConditionGroup['logic_op'] })}
              className="text-xs border border-gray-300 rounded px-2 py-1"
            >
              {LOGIC_OPS.map((op) => <option key={op} value={op}>{op}</option>)}
            </select>
            <button
              type="button"
              onClick={() => removeGroup(gIdx)}
              className="text-xs text-red-600 hover:underline ml-auto"
            >
              Удалить группу
            </button>
          </div>
          <div className="space-y-2">
            {g.conditions.map((c, cIdx) => (
              <div key={cIdx} className="flex items-center gap-2">
                <select
                  value={c.type}
                  onChange={(e) => updateCondition(gIdx, cIdx, { type: e.target.value as RouteCondition['type'] })}
                  className="text-xs border border-gray-300 rounded px-2 py-1"
                >
                  {TYPES.map((t) => <option key={t} value={t}>{t}</option>)}
                </select>
                <input
                  type="text"
                  value={c.value}
                  onChange={(e) => updateCondition(gIdx, cIdx, { value: e.target.value })}
                  className="flex-1 text-sm border border-gray-300 rounded px-2 py-1"
                  placeholder={c.type === 'country' ? 'RU, KZ, UZ ...' : c.type === 'regex' ? '^7.*' : ''}
                />
                <button
                  type="button"
                  onClick={() => removeCondition(gIdx, cIdx)}
                  className="text-xs text-red-600 hover:underline"
                  aria-label="Удалить условие"
                >
                  ×
                </button>
              </div>
            ))}
            <button
              type="button"
              onClick={() => addCondition(gIdx)}
              className="text-xs text-blue-600 hover:underline"
            >
              + Условие
            </button>
          </div>
        </div>
      ))}
      <button
        type="button"
        onClick={addGroup}
        className="text-sm text-blue-600 hover:underline"
      >
        + Группа условий
      </button>
    </div>
  );
}
