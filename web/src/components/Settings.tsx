import { useEffect, useState } from 'react';
import { toast } from 'sonner';
import { config, saveSettings, sync } from '../api/client';
import styles from './Settings.module.css';

export function Settings() {
  const [mode, setMode] = useState('wild-west');
  const [signing, setSigning] = useState(false);
  const [rigHandle, setRigHandle] = useState('');
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [syncing, setSyncing] = useState(false);

  useEffect(() => {
    (async () => {
      try {
        const cfg = await config();
        setRigHandle(cfg.rig_handle);
        setMode(cfg.mode || 'wild-west');
      } catch (e) {
        toast.error(e instanceof Error ? e.message : 'Failed to load config');
      } finally {
        setLoading(false);
      }
    })();
  }, []);

  const handleSave = async () => {
    setSaving(true);
    try {
      await saveSettings({ mode, signing });
      toast.success('Settings saved');
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Failed to save');
    } finally {
      setSaving(false);
    }
  };

  const handleSync = async () => {
    setSyncing(true);
    try {
      await sync();
      toast.success('Sync complete');
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Sync failed');
    } finally {
      setSyncing(false);
    }
  };

  if (loading) return <p className={styles.loadingText}>Loading...</p>;

  return (
    <div className={styles.page}>
      <h2 className={styles.heading}>Settings</h2>

      <div className={styles.section}>
        <h3 className={styles.sectionTitle}>Federation</h3>

        <div className={styles.field}>
          <div>
            <span className={styles.label}>Mode</span>
            <span className={styles.labelHint}>
              wild-west: direct writes &middot; pr: branch-based
            </span>
          </div>
          <select
            className={styles.select}
            value={mode}
            onChange={(e) => setMode(e.target.value)}
          >
            <option value="wild-west">wild-west</option>
            <option value="pr">pr</option>
          </select>
        </div>

        <div className={styles.field}>
          <div>
            <span className={styles.label}>Signing</span>
            <span className={styles.labelHint}>Cryptographically sign mutations</span>
          </div>
          <input
            type="checkbox"
            className={styles.toggle}
            checked={signing}
            onChange={(e) => setSigning(e.target.checked)}
          />
        </div>
      </div>

      <div className={styles.section}>
        <h3 className={styles.sectionTitle}>Identity</h3>
        <div className={styles.field}>
          <span className={styles.label}>Rig Handle</span>
          <span className={styles.configInfo}>{rigHandle || '-'}</span>
        </div>
      </div>

      <div className={styles.actions}>
        <button
          className={styles.saveBtn}
          onClick={handleSave}
          disabled={saving}
        >
          {saving ? 'Saving...' : 'Save'}
        </button>
        <button
          className={styles.syncBtn}
          onClick={handleSync}
          disabled={syncing}
        >
          {syncing ? 'Syncing...' : 'Sync'}
        </button>
      </div>
    </div>
  );
}
