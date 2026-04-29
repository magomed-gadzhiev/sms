import type { CreateProviderRequest } from '../../../api/client';

interface Props {
  data: Partial<CreateProviderRequest>;
  onChange: (updates: Partial<CreateProviderRequest>) => void;
}

export function Step3Params({ data, onChange }: Props) {
  return (
    <div>
      <h3>Параметры</h3>
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: 12, marginBottom: 16 }}>
        <label>Макс. одновременных подключений<br />
          <input
            type="number"
            min={1}
            max={100}
            value={data.max_connections ?? 1}
            onChange={e => onChange({ max_connections: parseInt(e.target.value) || 1 })}
            style={{ width: '100%', padding: '8px', marginTop: 4 }}
          />
        </label>
        <label>Размер окна (window)<br />
          <input
            type="number"
            min={1}
            max={1000}
            value={data.window_size ?? 10}
            onChange={e => onChange({ window_size: parseInt(e.target.value) || 10 })}
            style={{ width: '100%', padding: '8px', marginTop: 4 }}
          />
        </label>
        <label>Лимит TPS (сообщений/сек)<br />
          <input
            type="number"
            min={1}
            value={data.tps_limit ?? 100}
            onChange={e => onChange({ tps_limit: parseInt(e.target.value) || 100 })}
            style={{ width: '100%', padding: '8px', marginTop: 4 }}
          />
        </label>
      </div>
      <p style={{ color: '#666', fontSize: 13 }}>
        <strong>Макс. подключений</strong> — число одновременных SMPP-сессий (bind).<br />
        <strong>Размер окна</strong> — число неподтверждённых сообщений на одно подключение.<br />
        <strong>Лимит TPS</strong> — максимум сообщений в секунду через это подключение.
      </p>
    </div>
  );
}
