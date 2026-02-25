import styles from "./StatusBadge.module.css";

export function StatusBadge({ status }: { status: string }) {
  return (
    <span className={styles.badge} data-status={status}>
      {status.replace("_", " ")}
    </span>
  );
}
