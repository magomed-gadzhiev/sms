import type { CreateProviderRequest } from '../../../api/client';

interface Props {
  data: Partial<CreateProviderRequest>;
}

const BIND_LABELS: Record<number, string> = { 0: 'Transceiver', 1: 'Transmitter', 2: 'Receiver' };

function Row({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <tr>
      <td style={{ padding: '6px 8px', color: '#666', width: 160 }}>{label}</td>
      <td style={{ padding: '6px 8px' }}>{value}</td>
    </tr>
  );
}

export function Step6Summary({ data }: Props) {
  return (
    <div>
      <h3>Summary</h3>
      <p style={{ color: '#666' }}>Review your configuration before creating the provider.</p>
      <table style={{ width: '100%', borderCollapse: 'collapse', marginBottom: 16 }}>
        <tbody>
          <Row label="Name" value={data.name} />
          {data.description && <Row label="Description" value={data.description} />}
          {data.tags && data.tags.length > 0 && <Row label="Tags" value={data.tags.join(', ')} />}
          <Row label="Host" value={`${data.host}:${data.port}`} />
          <Row label="System ID" value={data.system_id} />
          <Row label="Bind Type" value={BIND_LABELS[data.bind_type ?? 0]} />
          <Row label="Max Connections" value={data.max_connections ?? 1} />
          <Row label="Window Size" value={data.window_size ?? 10} />
          <Row label="TPS Limit" value={data.tps_limit ?? 100} />
          {data.routing_rules && data.routing_rules.length > 0 && (
            <Row
              label="Routing Rules"
              value={data.routing_rules.map((r, i) => (
                <div key={i}>{r.pattern} (priority: {r.priority})</div>
              ))}
            />
          )}
        </tbody>
      </table>
    </div>
  );
}
