import type { CreateProviderRequest, RoutingRule } from '../../../api/client';

interface Props {
  data: Partial<CreateProviderRequest>;
  onChange: (updates: Partial<CreateProviderRequest>) => void;
}

export function Step5Routing({ data, onChange }: Props) {
  const rules = data.routing_rules ?? [];

  function addRule() {
    onChange({ routing_rules: [...rules, { pattern: '', priority: rules.length + 1 }] });
  }

  function removeRule(index: number) {
    onChange({ routing_rules: rules.filter((_, i) => i !== index) });
  }

  function updateRule(index: number, updates: Partial<RoutingRule>) {
    onChange({
      routing_rules: rules.map((r, i) => i === index ? { ...r, ...updates } : r),
    });
  }

  return (
    <div>
      <h3>Routing Rules</h3>
      <p style={{ color: '#666', fontSize: 13 }}>
        Define number patterns to route through this provider. Leave empty to use as default.
      </p>

      {rules.map((rule, i) => (
        <div key={i} style={{ display: 'flex', gap: 8, marginBottom: 8, alignItems: 'center' }}>
          <input
            value={rule.pattern}
            onChange={e => updateRule(i, { pattern: e.target.value })}
            placeholder="^7.*"
            style={{ flex: 1, padding: '6px 8px' }}
          />
          <input
            type="number"
            value={rule.priority}
            onChange={e => updateRule(i, { priority: parseInt(e.target.value) || 1 })}
            placeholder="Priority"
            style={{ width: 80, padding: '6px 8px' }}
          />
          <button type="button" onClick={() => removeRule(i)} style={{ color: '#d32f2f', border: 'none', background: 'none', cursor: 'pointer', fontSize: 16 }}>
            ×
          </button>
        </div>
      ))}

      <button
        type="button"
        onClick={addRule}
        style={{ padding: '6px 16px', marginTop: 8, cursor: 'pointer' }}
      >
        + Add Rule
      </button>
    </div>
  );
}
