import { useState, useEffect } from 'react';
import { analyticsApi, ApiError } from '../../api/client';

interface AnalyticsSummary {
  total_sent: number;
  total_delivered: number;
  total_failed: number;
  total_expired: number;
  delivery_rate: number;
  total_cost: string;
  currency: string;
}

interface TimelineEntry {
  period: string;
  sent: number;
  delivered: number;
  failed: number;
  delivery_rate: number;
}

interface CountryEntry {
  country: string;
  sent: number;
  delivered: number;
  failed: number;
  delivery_rate: number;
}

interface AnalyticsData {
  summary: AnalyticsSummary;
  timeline: TimelineEntry[];
  by_country: CountryEntry[];
}

const PERIODS = ['7d', '30d', '90d'] as const;

export function AnalyticsPage() {
  const [data, setData] = useState<AnalyticsData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const [period, setPeriod] = useState<string>('7d');
  const [dateFrom, setDateFrom] = useState('');
  const [dateTo, setDateTo] = useState('');
  const [groupBy, setGroupBy] = useState('day');
  const [useCustomDates, setUseCustomDates] = useState(false);

  async function loadAnalytics() {
    setLoading(true);
    setError('');
    try {
      const params: Record<string, string> = { group_by: groupBy };
      if (useCustomDates && dateFrom && dateTo) {
        params.date_from = dateFrom;
        params.date_to = dateTo;
      } else {
        params.period = period;
      }
      const resp = await analyticsApi.get(params);
      setData(resp as AnalyticsData);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Failed to load analytics');
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    loadAnalytics();
  }, [period, dateFrom, dateTo, groupBy, useCustomDates]);

  return (
    <div style={{ maxWidth: 1000 }}>
      <h2>Analytics</h2>

      {/* Period selector + date filters */}
      <fieldset style={{ border: 'none', padding: 0, margin: '0 0 16px' }}>
        <legend style={{ fontWeight: 'bold', marginBottom: 8 }}>Filters</legend>
        <div style={{ display: 'flex', gap: 8, alignItems: 'center', flexWrap: 'wrap' }}>
          {PERIODS.map((p) => (
            <button
              key={p}
              onClick={() => {
                setPeriod(p);
                setUseCustomDates(false);
              }}
              style={{
                padding: '6px 16px',
                fontWeight: !useCustomDates && period === p ? 'bold' : 'normal',
                background: !useCustomDates && period === p ? '#1976d2' : '#e0e0e0',
                color: !useCustomDates && period === p ? '#fff' : '#333',
                border: 'none',
                borderRadius: 4,
                cursor: 'pointer',
              }}
            >
              {p}
            </button>
          ))}

          <span style={{ margin: '0 8px', color: '#767676' }}>or</span>

          <label style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
            From:
            <input
              type="date"
              value={dateFrom}
              onChange={(e) => {
                setDateFrom(e.target.value);
                setUseCustomDates(true);
              }}
            />
          </label>
          <label style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
            To:
            <input
              type="date"
              value={dateTo}
              onChange={(e) => {
                setDateTo(e.target.value);
                setUseCustomDates(true);
              }}
            />
          </label>

          <span style={{ margin: '0 8px', color: '#767676' }}>|</span>

          <label style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
            Group by:
            <select value={groupBy} onChange={(e) => setGroupBy(e.target.value)}>
              <option value="day">Day</option>
              <option value="week">Week</option>
              <option value="country">Country</option>
            </select>
          </label>
        </div>
      </fieldset>

      {error && <p style={{ color: '#d32f2f' }}>{error}</p>}
      {loading && <div role="status">Loading analytics...</div>}

      {data && !loading && (
        <>
          {/* Summary cards */}
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: 16, marginBottom: 24 }}>
            {[
              { label: 'Sent', value: data.summary.total_sent },
              { label: 'Delivered', value: data.summary.total_delivered },
              { label: 'Failed', value: data.summary.total_failed },
              { label: 'Delivery Rate', value: `${data.summary.delivery_rate}%` },
              {
                label: 'Total Cost',
                value: data.summary.total_cost
                  ? `${data.summary.total_cost} ${data.summary.currency}`
                  : 'N/A',
              },
            ].map((card) => (
              <div
                key={card.label}
                style={{
                  border: '1px solid #ddd',
                  borderRadius: 8,
                  padding: 16,
                  minWidth: 160,
                  textAlign: 'center',
                }}
              >
                <div style={{ fontSize: 14, color: '#666', marginBottom: 8 }}>{card.label}</div>
                <div style={{ fontSize: 24, fontWeight: 'bold' }}>{card.value}</div>
              </div>
            ))}
          </div>

          {/* Timeline table */}
          {data.timeline.length > 0 && (
            <div style={{ marginBottom: 24 }}>
              <h3>Timeline</h3>
              <table style={{ width: '100%', borderCollapse: 'collapse' }}>
                <caption style={{ textAlign: 'left', marginBottom: 8, fontWeight: 'bold' }}>Message statistics by period</caption>
                <thead>
                  <tr style={{ borderBottom: '2px solid #ddd', textAlign: 'left' }}>
                    <th style={{ padding: 8 }}>Period</th>
                    <th style={{ padding: 8 }}>Sent</th>
                    <th style={{ padding: 8 }}>Delivered</th>
                    <th style={{ padding: 8 }}>Failed</th>
                    <th style={{ padding: 8 }}>Delivery Rate</th>
                  </tr>
                </thead>
                <tbody>
                  {data.timeline.map((row, i) => (
                    <tr key={i} style={{ borderBottom: '1px solid #eee' }}>
                      <td style={{ padding: 8 }}>{row.period}</td>
                      <td style={{ padding: 8 }}>{row.sent}</td>
                      <td style={{ padding: 8 }}>{row.delivered}</td>
                      <td style={{ padding: 8 }}>{row.failed}</td>
                      <td style={{ padding: 8 }}>{row.delivery_rate}%</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}

          {/* Country breakdown table */}
          {data.by_country.length > 0 && (
            <div>
              <h3>By Country</h3>
              <table style={{ width: '100%', borderCollapse: 'collapse' }}>
                <caption style={{ textAlign: 'left', marginBottom: 8, fontWeight: 'bold' }}>Message statistics by country</caption>
                <thead>
                  <tr style={{ borderBottom: '2px solid #ddd', textAlign: 'left' }}>
                    <th style={{ padding: 8 }}>Country</th>
                    <th style={{ padding: 8 }}>Sent</th>
                    <th style={{ padding: 8 }}>Delivered</th>
                    <th style={{ padding: 8 }}>Failed</th>
                    <th style={{ padding: 8 }}>Delivery Rate</th>
                  </tr>
                </thead>
                <tbody>
                  {data.by_country.map((row, i) => (
                    <tr key={i} style={{ borderBottom: '1px solid #eee' }}>
                      <td style={{ padding: 8 }}>{row.country}</td>
                      <td style={{ padding: 8 }}>{row.sent}</td>
                      <td style={{ padding: 8 }}>{row.delivered}</td>
                      <td style={{ padding: 8 }}>{row.failed}</td>
                      <td style={{ padding: 8 }}>{row.delivery_rate}%</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </>
      )}
    </div>
  );
}
