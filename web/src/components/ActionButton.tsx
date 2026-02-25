import { useState } from 'react';
import styles from './ActionButton.module.css';

interface ActionButtonProps {
  action: string;
  onAction: () => Promise<void>;
}

export function ActionButton({ action, onAction }: ActionButtonProps) {
  const [loading, setLoading] = useState(false);

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
      className={styles.button}
      data-action={action}
      onClick={handleClick}
      disabled={loading}
    >
      {loading ? `${action}...` : action}
    </button>
  );
}
