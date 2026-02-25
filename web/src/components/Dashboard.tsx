import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { dashboard } from '../api/client';
import type { DashboardResponse, WantedSummary } from '../api/types';
import { ayu, statusColor } from '../styles/theme';
import { StatusBadge } from './StatusBadge';
import { PriorityBadge } from './PriorityBadge';

export function Dashboard() {
  const [data, setData] = useState<DashboardResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    (async () => {
      try {
        setData(await dashboard());
      } catch (e) {
        setError(e instanceof Error ? e.message : 'Failed to load');
      } finally {
        setLoading(false);
      }
    })();
  }, []);

  if (loading) return <p style={{ color: ayu.dim }}>Loading...</p>;
  if (error) return <p style={{ color: ayu.accent }}>{error}</p>;
  if (!data) return <p style={{ color: ayu.dim }}>No data.</p>;

  return (
    <div>
      <h2 style={{ color: ayu.fg, fontSize: '20px', fontWeight: 700, marginBottom: '20px' }}>
        My Dashboard
      </h2>
      <DashboardSection title="Claimed" color={statusColor.claimed} items={data.claimed} />
      <DashboardSection title="In Review" color={statusColor.in_review} items={data.in_review} />
      <DashboardSection title="Completed" color={statusColor.completed} items={data.completed} />
    </div>
  );
}

function DashboardSection({
  title,
  color,
  items,
}: {
  title: string;
  color: string;
  items: WantedSummary[];
}) {
  return (
    <div style={{ marginBottom: '24px' }}>
      <h3
        style={{
          color,
          fontSize: '14px',
          fontWeight: 700,
          marginBottom: '8px',
          paddingBottom: '6px',
          borderBottom: `2px solid ${ayu.border}`,
          letterSpacing: '0.05em',
          textTransform: 'uppercase',
        }}
      >
        {title} ({items.length})
      </h3>
      {items.length === 0 ? (
        <p style={{ color: ayu.dim, fontSize: '14px' }}>None</p>
      ) : (
        <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '14px' }}>
          <tbody>
            {items.map((item) => (
              <tr
                key={item.id}
                style={{
                  borderBottom: `1px solid ${ayu.border}`,
                  cursor: 'pointer',
                }}
                onMouseOver={(e) => {
                  e.currentTarget.style.background = ayu.surface;
                }}
                onMouseOut={(e) => {
                  e.currentTarget.style.background = 'transparent';
                }}
              >
                <td style={{ padding: '6px 12px', width: '60px' }}>
                  <PriorityBadge priority={item.priority} />
                </td>
                <td style={{ padding: '6px 12px' }}>
                  <Link
                    to={`/wanted/${item.id}`}
                    style={{ color: ayu.fg, textDecoration: 'none' }}
                  >
                    {item.title}
                  </Link>
                </td>
                <td style={{ padding: '6px 12px', width: '100px' }}>
                  <StatusBadge status={item.status} />
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}
