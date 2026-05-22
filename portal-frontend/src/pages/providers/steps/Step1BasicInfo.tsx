import type { CreateProviderRequest } from '../../../api/client';

interface Props {
  data: Partial<CreateProviderRequest>;
  onChange: (updates: Partial<CreateProviderRequest>) => void;
}

export function Step1BasicInfo({ data, onChange }: Props) {
  return (
    <div>
      <h3>Основное</h3>
      <div style={{ marginBottom: 16 }}>
        <label>Название *<br />
          <input
            value={data.name ?? ''}
            onChange={e => onChange({ name: e.target.value })}
            placeholder="Моё SMPP-подключение"
            style={{ width: '100%', padding: '8px', marginTop: 4 }}
          />
        </label>
      </div>
      <div style={{ marginBottom: 16 }}>
        <label>Описание<br />
          <textarea
            value={data.description ?? ''}
            onChange={e => onChange({ description: e.target.value })}
            placeholder="Необязательное описание"
            rows={3}
            style={{ width: '100%', padding: '8px', marginTop: 4 }}
          />
        </label>
      </div>
      <div style={{ marginBottom: 16 }}>
        <label>Метки (через запятую)<br />
          <input
            value={(data.tags ?? []).join(', ')}
            onChange={e => onChange({ tags: e.target.value.split(',').map(t => t.trim()).filter(Boolean) })}
            placeholder="prod, eu-west"
            style={{ width: '100%', padding: '8px', marginTop: 4 }}
          />
        </label>
      </div>
    </div>
  );
}
