import type { CreateProviderRequest } from '../../../api/client';

interface Props {
  data: Partial<CreateProviderRequest>;
  onChange: (updates: Partial<CreateProviderRequest>) => void;
}

const BIND_TYPES = [
  { value: 0, label: 'Двусторонний (TRX)' },
  { value: 1, label: 'Только отправка (TX)' },
  { value: 2, label: 'Только приём (RX)' },
];

export function Step2Connection({ data, onChange }: Props) {
  return (
    <div>
      <h3>Подключение</h3>
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 120px', gap: 12, marginBottom: 16 }}>
        <label>Хост *<br />
          <input
            value={data.host ?? ''}
            onChange={e => onChange({ host: e.target.value })}
            placeholder="smpp.example.com"
            style={{ width: '100%', padding: '8px', marginTop: 4 }}
          />
        </label>
        <label>Порт *<br />
          <input
            type="number"
            value={data.port ?? ''}
            onChange={e => onChange({ port: parseInt(e.target.value) || 0 })}
            placeholder="2775"
            style={{ width: '100%', padding: '8px', marginTop: 4 }}
          />
        </label>
      </div>
      <div style={{ marginBottom: 16 }}>
        <label>Идентификатор системы (System ID) *<br />
          <input
            value={data.system_id ?? ''}
            onChange={e => onChange({ system_id: e.target.value })}
            placeholder="smppclient1"
            style={{ width: '100%', padding: '8px', marginTop: 4 }}
          />
        </label>
      </div>
      <div style={{ marginBottom: 16 }}>
        <label>Пароль *<br />
          <input
            type="password"
            value={data.password ?? ''}
            onChange={e => onChange({ password: e.target.value })}
            placeholder="••••••••"
            style={{ width: '100%', padding: '8px', marginTop: 4 }}
          />
        </label>
      </div>
      <div style={{ marginBottom: 16 }}>
        <label>Режим подключения<br />
          <select
            value={data.bind_type ?? 0}
            onChange={e => onChange({ bind_type: parseInt(e.target.value) })}
            style={{ padding: '8px', marginTop: 4 }}
          >
            {BIND_TYPES.map(bt => (
              <option key={bt.value} value={bt.value}>{bt.label}</option>
            ))}
          </select>
        </label>
      </div>
    </div>
  );
}
