import type { CreateProviderRequest } from '../../../api/client';

interface Props {
  data: Partial<CreateProviderRequest>;
  onChange: (updates: Partial<CreateProviderRequest>) => void;
}

export function Step1BasicInfo({ data, onChange }: Props) {
  return (
    <div>
      <h3>Basic Information</h3>
      <div style={{ marginBottom: 16 }}>
        <label>Name *<br />
          <input
            value={data.name ?? ''}
            onChange={e => onChange({ name: e.target.value })}
            placeholder="My SMPP Provider"
            style={{ width: '100%', padding: '8px', marginTop: 4 }}
          />
        </label>
      </div>
      <div style={{ marginBottom: 16 }}>
        <label>Description<br />
          <textarea
            value={data.description ?? ''}
            onChange={e => onChange({ description: e.target.value })}
            placeholder="Optional description"
            rows={3}
            style={{ width: '100%', padding: '8px', marginTop: 4 }}
          />
        </label>
      </div>
      <div style={{ marginBottom: 16 }}>
        <label>Tags (comma-separated)<br />
          <input
            value={(data.tags ?? []).join(', ')}
            onChange={e => onChange({ tags: e.target.value.split(',').map(t => t.trim()).filter(Boolean) })}
            placeholder="production, eu-west"
            style={{ width: '100%', padding: '8px', marginTop: 4 }}
          />
        </label>
      </div>
    </div>
  );
}
