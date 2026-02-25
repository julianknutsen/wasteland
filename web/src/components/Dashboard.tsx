import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { toast } from 'sonner';
import { dashboard } from '../api/client';
import type { DashboardResponse, WantedSummary } from '../api/types';
import { StatusBadge } from './StatusBadge';
import { PriorityBadge } from './PriorityBadge';
import { SkeletonRows } from './Skeleton';
import { EmptyState } from './EmptyState';
import styles from './Dashboard.module.css';

export function Dashboard() {
  const [data, setData] = useState<DashboardResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    (async () => {
      try {
        setData(await dashboard());
      } catch (e) {
        const msg = e instanceof Error ? e.message : 'Failed to load';
        setError(msg);
        toast.error(msg);
      } finally {
        setLoading(false);
      }
    })();
  }, []);

  if (loading) return (
    <div className={styles.page}>
      <h2 className={styles.heading}>My Dashboard</h2>
      {['Claimed', 'In Review', 'Completed'].map((s) => (
        <div key={s} className={styles.section}>
          <h3 className={styles.sectionTitle}>{s}</h3>
          <SkeletonRows count={2} />
        </div>
      ))}
    </div>
  );
  if (error) return <p className={styles.errorText}>{error}</p>;
  if (!data) return <p className={styles.loadingText}>No data.</p>;

  return (
    <div className={styles.page}>
      <h2 className={styles.heading}>My Dashboard</h2>
      <DashboardSection title="Claimed" status="claimed" items={data.claimed} />
      <DashboardSection title="In Review" status="in_review" items={data.in_review} />
      <DashboardSection title="Completed" status="completed" items={data.completed} />
    </div>
  );
}

function DashboardSection({
  title,
  status,
  items,
}: {
  title: string;
  status: string;
  items: WantedSummary[];
}) {
  return (
    <div className={styles.section}>
      <h3 className={styles.sectionTitle} data-status={status}>
        {title} ({items.length})
      </h3>
      {items.length === 0 ? (
        <EmptyState
          title={`No ${title.toLowerCase()} items`}
          description={`Items you've ${status === 'claimed' ? 'claimed' : status === 'in_review' ? 'submitted for review' : 'completed'} will appear here.`}
        />
      ) : (
        <table className={styles.table}>
          <tbody>
            {items.map((item) => (
              <tr key={item.id} className={styles.row}>
                <td className={styles.cellPriority}>
                  <PriorityBadge priority={item.priority} />
                </td>
                <td className={styles.cellTitle}>
                  <Link to={`/wanted/${item.id}`} className={styles.titleLink}>
                    {item.title}
                  </Link>
                </td>
                <td className={styles.cellStatus}>
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
