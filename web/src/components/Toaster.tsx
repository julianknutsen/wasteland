import { Toaster as SonnerToaster } from 'sonner';
import styles from './Toaster.module.css';

export function Toaster() {
  return (
    <SonnerToaster
      className={styles.toaster}
      position="bottom-right"
      toastOptions={{
        style: {
          background: 'var(--surface)',
          color: 'var(--fg)',
          border: '1px solid var(--border)',
          fontFamily: "var(--font-body)",
          fontSize: 'var(--text-base)',
        },
      }}
    />
  );
}
