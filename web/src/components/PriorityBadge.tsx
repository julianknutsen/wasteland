import { ayu } from '../styles/theme';

const priorityColors: Record<number, string> = {
  0: ayu.accent,
  1: ayu.brass,
  2: ayu.copper,
  3: ayu.green,
  4: ayu.dim,
};

export function PriorityBadge({ priority }: { priority: number }) {
  const color = priorityColors[priority] || ayu.dim;
  return (
    <span
      style={{
        display: 'inline-block',
        padding: '2px 6px',
        borderRadius: '4px',
        fontSize: '11px',
        fontWeight: 700,
        fontFamily: "'Cinzel', 'Times New Roman', serif",
        letterSpacing: '0.05em',
        color,
        border: `1px solid ${color}`,
      }}
    >
      P{priority}
    </span>
  );
}
