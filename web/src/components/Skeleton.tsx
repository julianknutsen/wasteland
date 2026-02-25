import styles from "./Skeleton.module.css";

interface SkeletonProps {
  width?: string;
  height?: string;
  className?: string;
}

export function Skeleton({ width, height, className }: SkeletonProps) {
  return <div className={`${styles.skeleton} ${className ?? ""}`} style={{ width, height }} />;
}

export function SkeletonLine({ width }: { width?: string }) {
  return <div className={styles.line} style={{ width: width ?? "100%" }} />;
}

export function SkeletonBadge() {
  return <div className={styles.badge} />;
}

export function SkeletonRow() {
  return (
    <div className={styles.row}>
      <SkeletonBadge />
      <SkeletonLine width="60%" />
      <SkeletonBadge />
    </div>
  );
}

export function SkeletonRows({ count }: { count: number }) {
  return (
    <>
      {Array.from({ length: count }, (_, i) => (
        <SkeletonRow key={i} />
      ))}
    </>
  );
}

export function SkeletonBlock() {
  return <div className={styles.block} />;
}
