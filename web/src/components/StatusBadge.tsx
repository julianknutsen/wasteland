import { statusColor, ayu } from '../styles/theme';

export function StatusBadge({ status }: { status: string }) {
  const color = statusColor[status] || ayu.dim;
  return (
    <span
      style={{
        display: 'inline-block',
        padding: '2px 8px',
        borderRadius: '4px',
        fontSize: '11px',
        fontWeight: 700,
        fontFamily: "'Cinzel', 'Times New Roman', serif",
        letterSpacing: '0.05em',
        textTransform: 'uppercase',
        color: '#fff',
        backgroundColor: color,
      }}
    >
      {status.replace('_', ' ')}
    </span>
  );
}
