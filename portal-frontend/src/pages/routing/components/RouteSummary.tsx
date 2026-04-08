import type { RouteFormData } from '../types';
import { WEEKDAY_LABELS, WEEKDAY_BITS, LOGIC_OP_LABELS, CONDITION_TYPE_LABELS } from '../types';

interface RouteSummaryProps {
  data: RouteFormData;
  providerName: string;
}

export function RouteSummary({ data, providerName }: RouteSummaryProps) {
  const routeTypeLabel = { sms: 'SMS', hlr: 'HLR', max: 'MAX' }[data.route_type] || data.route_type;
  const statusLabel = data.status === 'active' ? 'Активный' : 'Черновик';

  return (
    <div className="flex flex-col gap-4 text-sm">
      <h3 className="font-semibold text-gray-900">Предпросмотр</h3>

      <div className="flex flex-col gap-2">
        <SummaryRow label="Тип" value={routeTypeLabel} />
        <SummaryRow label="Приоритет" value={String(data.priority)} />
        <SummaryRow label="Доля" value={`${data.share}%`} />
        <SummaryRow label="Провайдер" value={providerName || '---'} />
        <SummaryRow label="Статус" value={statusLabel} />
      </div>

      {data.condition_groups.length > 0 && (
        <div>
          <div className="font-medium text-gray-700 mb-1">Условия</div>
          <div className="flex flex-col gap-1">
            {data.condition_groups.map((group, gi) => (
              <div key={gi} className="flex flex-wrap items-center gap-1">
                <span className="text-xs font-semibold text-gray-500">
                  {LOGIC_OP_LABELS[group.logic_op]}
                </span>
                {group.conditions.map((cond, ci) => (
                  <span
                    key={ci}
                    className="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium bg-blue-50 text-blue-600"
                  >
                    {CONDITION_TYPE_LABELS[cond.type]}: {cond.value || '...'}
                  </span>
                ))}
              </div>
            ))}
          </div>
        </div>
      )}

      {data.schedules.length > 0 && data.schedules[0] && (
        <div>
          <div className="font-medium text-gray-700 mb-1">Расписание</div>
          {data.schedules.map((sch, i) => {
            const days = WEEKDAY_LABELS.filter((_, di) => (sch.weekdays & WEEKDAY_BITS[di]) !== 0);
            return (
              <div key={i} className="text-xs text-gray-600">
                <div>{days.join(', ')}</div>
                {(sch.time_from || sch.time_to) && (
                  <div>{sch.time_from || '00:00'} - {sch.time_to || '23:59'}</div>
                )}
                {(sch.date_from || sch.date_to) && (
                  <div>{sch.date_from || '...'} - {sch.date_to || '...'}</div>
                )}
                <div className="text-gray-400">{sch.timezone}</div>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}

function SummaryRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex justify-between">
      <span className="text-gray-500">{label}</span>
      <span className="font-medium text-gray-900">{value}</span>
    </div>
  );
}
