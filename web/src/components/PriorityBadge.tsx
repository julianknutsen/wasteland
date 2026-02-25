import styles from './PriorityBadge.module.css';

export function PriorityBadge({ priority }: { priority: number }) {
  return (
    <span className={styles.badge} data-priority={priority}>
      P{priority}
    </span>
  );
}
