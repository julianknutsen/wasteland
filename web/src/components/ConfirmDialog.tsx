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
        background: 'rgba(62,39,35,0.6)',
        zIndex: 100,
      }}
    >
      <div
        style={{
          background: ayu.surface,
          border: `2px solid ${ayu.border}`,
          borderRadius: '6px',
          padding: '24px',
          maxWidth: '400px',
          width: '90%',
          boxShadow: '0 4px 24px rgba(44,24,16,0.3)',
        }}
      >
        <p style={{ marginBottom: '16px', color: ayu.fg, fontSize: '15px' }}>{message}</p>
        <div style={{ display: 'flex', gap: '8px', justifyContent: 'flex-end' }}>
          <button
            onClick={onCancel}
            style={{
              padding: '6px 18px',
              borderRadius: '4px',
              border: `1px solid ${ayu.border}`,
              background: 'transparent',
              color: ayu.dim,
              cursor: 'pointer',
              fontFamily: "'Cinzel', 'Times New Roman', serif",
              fontSize: '12px',
              fontWeight: 600,
              letterSpacing: '0.05em',
              textTransform: 'uppercase',
            }}
          >
            Cancel
          </button>
          <button
            onClick={onConfirm}
            style={{
              padding: '6px 18px',
              borderRadius: '4px',
              border: `1px solid ${ayu.accent}`,
              background: ayu.accent,
              color: ayu.fgLight,
              cursor: 'pointer',
              fontFamily: "'Cinzel', 'Times New Roman', serif",
              fontSize: '12px',
              fontWeight: 600,
              letterSpacing: '0.05em',
              textTransform: 'uppercase',
            }}
          >
            Confirm
          </button>
        </div>
      </div>
    </div>
  );
}
