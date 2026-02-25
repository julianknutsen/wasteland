import { ayu } from '../styles/theme';

const priorityColors: Record<number, string> = {
  0: ayu.red,
  1: ayu.orange,
  2: ayu.accent,
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
        fontSize: '12px',
        fontWeight: 600,
        color,
        border: `1px solid ${color}`,
      }}
    >
      P{priority}
    </span>
  );
}
