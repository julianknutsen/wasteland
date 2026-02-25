import { useState } from 'react';
import { ayu, statusColor } from '../styles/theme';

const actionColors: Record<string, string> = {
  claim: statusColor.claimed,
  unclaim: statusColor.open,
  done: ayu.green,
  accept: ayu.green,
  reject: ayu.accent,
  close: statusColor.completed,
  delete: ayu.accent,
  'submit pr': ayu.brass,
  apply: ayu.green,
  discard: ayu.accent,
};

interface ActionButtonProps {
  action: string;
  onAction: () => Promise<void>;
}

export function ActionButton({ action, onAction }: ActionButtonProps) {
  const [loading, setLoading] = useState(false);
  const color = actionColors[action] || ayu.accent;

  const handleClick = async () => {
    setLoading(true);
    try {
      await onAction();
    } finally {
      setLoading(false);
    }
  };

  return (
    <button
      onClick={handleClick}
      disabled={loading}
      style={{
        padding: '6px 18px',
        borderRadius: '4px',
        border: `1px solid ${color}`,
        background: 'transparent',
        color,
        opacity: loading ? 0.5 : 1,
        cursor: loading ? 'wait' : 'pointer',
        fontFamily: "'Cinzel', 'Times New Roman', serif",
        fontSize: '12px',
        fontWeight: 600,
        letterSpacing: '0.05em',
        textTransform: 'uppercase',
      }}
    >
      {loading ? `${action}...` : action}
    </button>
  );
}
