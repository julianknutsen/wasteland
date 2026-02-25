import { statusColor, ayu } from '../styles/theme';

export function StatusBadge({ status }: { status: string }) {
  const color = statusColor[status] || ayu.dim;
  return (
    <span
      style={{
        display: 'inline-block',
        padding: '2px 8px',
        borderRadius: '4px',
        fontSize: '12px',
        fontWeight: 600,
        color: ayu.bg,
        backgroundColor: color,
      }}
    >
      {status.replace('_', ' ')}
    </span>
  );
}
