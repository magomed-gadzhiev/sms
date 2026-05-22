import type { CreateProviderRequest } from '../../../api/client';

interface Props {
  data: Partial<CreateProviderRequest>;
}

const BIND_LABELS: Record<number, string> = { 0: 'Двусторонний (TRX)', 1: 'Только отправка (TX)', 2: 'Только приём (RX)' };

function Row({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <tr>
      <td style={{ padding: '6px 8px', color: '#666', width: 200 }}>{label}</td>
      <td style={{ padding: '6px 8px' }}>{value}</td>
    </tr>
  );
}

export function Step6Summary({ data }: Props) {
  return (
    <div>
      <h3>Сводка</h3>
      <p style={{ color: '#666' }}>Проверьте параметры перед созданием подключения.</p>
      <table style={{ width: '100%', borderCollapse: 'collapse', marginBottom: 16 }}>
        <tbody>
          <Row label="Название" value={data.name} />
          {data.description && <Row label="Описание" value={data.description} />}
          {data.tags && data.tags.length > 0 && <Row label="Метки" value={data.tags.join(', ')} />}
          <Row label="Хост" value={`${data.host}:${data.port}`} />
          <Row label="System ID" value={data.system_id} />
          <Row label="Режим подключения" value={BIND_LABELS[data.bind_type ?? 0]} />
          <Row label="Макс. подключений" value={data.max_connections ?? 1} />
          <Row label="Размер окна" value={data.window_size ?? 10} />
          <Row label="Лимит TPS" value={data.tps_limit ?? 100} />
        </tbody>
      </table>
    </div>
  );
}
