import { useEffect, useState, useCallback } from 'react';
import { Link } from 'react-router-dom';
import { browse } from '../api/client';
import type { BrowseFilter, WantedSummary } from '../api/types';
import { ayu } from '../styles/theme';
import { StatusBadge } from './StatusBadge';
import { PriorityBadge } from './PriorityBadge';
import { FilterBar } from './FilterBar';

export function BrowseList() {
  const [items, setItems] = useState<WantedSummary[]>([]);
  const [filter, setFilter] = useState<BrowseFilter>({});
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const load = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const resp = await browse(filter);
      setItems(resp.items);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load');
    } finally {
      setLoading(false);
    }
  }, [filter]);

  useEffect(() => {
    load();
  }, [load]);

  return (
    <div>
      <div style={{ marginBottom: '16px', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <h2 style={{ color: ayu.fg, fontSize: '16px', fontWeight: 600 }}>Wanted Board</h2>
      </div>

      <FilterBar filter={filter} onChange={setFilter} />

      {error && <p style={{ color: ayu.red, marginTop: '12px' }}>{error}</p>}

      {loading ? (
        <p style={{ color: ayu.dim, marginTop: '12px' }}>Loading...</p>
      ) : items.length === 0 ? (
        <p style={{ color: ayu.dim, marginTop: '12px' }}>No items found.</p>
      ) : (
        <table
          style={{
            width: '100%',
            marginTop: '12px',
            borderCollapse: 'collapse',
            fontSize: '13px',
          }}
        >
          <thead>
            <tr style={{ borderBottom: `1px solid ${ayu.border}`, color: ayu.dim, textAlign: 'left' }}>
              <th style={{ padding: '8px 12px' }}>Priority</th>
              <th style={{ padding: '8px 12px' }}>Title</th>
              <th style={{ padding: '8px 12px' }}>Status</th>
              <th style={{ padding: '8px 12px' }}>Type</th>
              <th style={{ padding: '8px 12px' }}>Posted By</th>
              <th style={{ padding: '8px 12px' }}>Claimed By</th>
            </tr>
          </thead>
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
                <td style={{ padding: '8px 12px' }}>
                  <PriorityBadge priority={item.priority} />
                </td>
                <td style={{ padding: '8px 12px' }}>
                  <Link
                    to={`/wanted/${item.id}`}
                    style={{ color: ayu.fg, textDecoration: 'none' }}
                  >
                    {item.title}
                    {item.has_branch && (
                      <span style={{ color: ayu.purple, marginLeft: '6px', fontSize: '11px' }}>branch</span>
                    )}
                  </Link>
                </td>
                <td style={{ padding: '8px 12px' }}>
                  <StatusBadge status={item.status} />
                </td>
                <td style={{ padding: '8px 12px', color: ayu.dim }}>{item.type || '-'}</td>
                <td style={{ padding: '8px 12px', color: ayu.dim }}>{item.posted_by || '-'}</td>
                <td style={{ padding: '8px 12px', color: ayu.dim }}>{item.claimed_by || '-'}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}
