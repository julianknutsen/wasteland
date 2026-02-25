import { ayu } from '../styles/theme';

interface ConfirmDialogProps {
  message: string;
  onConfirm: () => void;
  onCancel: () => void;
}

export function ConfirmDialog({ message, onConfirm, onCancel }: ConfirmDialogProps) {
  return (
    <div
      style={{
        position: 'fixed',
        inset: 0,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        background: 'rgba(0,0,0,0.6)',
        zIndex: 100,
      }}
    >
      <div
        style={{
          background: ayu.surface,
          border: `1px solid ${ayu.border}`,
          borderRadius: '8px',
          padding: '24px',
          maxWidth: '400px',
          width: '90%',
        }}
      >
        <p style={{ marginBottom: '16px', color: ayu.fg }}>{message}</p>
        <div style={{ display: 'flex', gap: '8px', justifyContent: 'flex-end' }}>
          <button
            onClick={onCancel}
            style={{
              padding: '6px 16px',
              borderRadius: '4px',
              border: `1px solid ${ayu.border}`,
              background: 'transparent',
              color: ayu.dim,
              cursor: 'pointer',
            }}
          >
            Cancel
          </button>
          <button
            onClick={onConfirm}
            style={{
              padding: '6px 16px',
              borderRadius: '4px',
              border: `1px solid ${ayu.red}`,
              background: ayu.red,
              color: ayu.bg,
              cursor: 'pointer',
            }}
          >
            Confirm
          </button>
        </div>
      </div>
    </div>
  );
}
