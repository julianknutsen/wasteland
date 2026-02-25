import styles from './EmptyState.module.css';

interface EmptyStateProps {
  title: string;
  description: string;
  ctaLabel?: string;
  onCta?: () => void;
}

export function EmptyState({ title, description, ctaLabel, onCta }: EmptyStateProps) {
  return (
    <div className={styles.container}>
      <h3 className={styles.title}>{title}</h3>
      <p className={styles.description}>{description}</p>
      {ctaLabel && onCta && (
        <button className={styles.ctaBtn} onClick={onCta}>
          {ctaLabel}
        </button>
      )}
    </div>
  );
}
