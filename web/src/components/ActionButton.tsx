import { useState } from 'react';
import { ayu, statusColor } from '../styles/theme';

const actionColors: Record<string, string> = {
  claim: statusColor.claimed,
  unclaim: statusColor.open,
  reject: ayu.red,
  close: statusColor.completed,
  delete: ayu.red,
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
        padding: '6px 16px',
        borderRadius: '4px',
        border: `1px solid ${color}`,
        background: 'transparent',
        color,
        opacity: loading ? 0.5 : 1,
        cursor: loading ? 'wait' : 'pointer',
      }}
    >
      {loading ? `${action}...` : action}
    </button>
  );
}
