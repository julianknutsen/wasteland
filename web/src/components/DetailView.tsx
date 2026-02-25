import { useEffect, useState, useCallback } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { detail, claim, unclaim, reject, close, deleteItem } from '../api/client';
import type { DetailResponse } from '../api/types';
import { ayu, statusColor } from '../styles/theme';
import { StatusBadge } from './StatusBadge';
import { PriorityBadge } from './PriorityBadge';
import { ActionButton } from './ActionButton';
import { ConfirmDialog } from './ConfirmDialog';

const actionHandlers: Record<string, (id: string) => Promise<unknown>> = {
  claim: (id) => claim(id),
  unclaim: (id) => unclaim(id),
  reject: (id) => reject(id),
  close: (id) => close(id),
  delete: (id) => deleteItem(id),
};

const destructiveActions = new Set(['delete', 'close', 'reject']);

export function DetailView() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [data, setData] = useState<DetailResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [confirm, setConfirm] = useState<string | null>(null);

  const load = useCallback(async () => {
    if (!id) return;
    setLoading(true);
    setError('');
    try {
      setData(await detail(id));
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load');
    } finally {
      setLoading(false);
    }
  }, [id]);

  useEffect(() => {
    load();
  }, [load]);

  const handleAction = async (action: string) => {
    if (!id) return;
    const handler = actionHandlers[action];
    if (!handler) return;
    try {
      await handler(id);
      if (action === 'delete') {
        navigate('/');
      } else {
        await load();
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : `Failed to ${action}`);
    }
  };

  const onActionClick = (action: string) => {
    if (destructiveActions.has(action)) {
      setConfirm(action);
    } else {
      handleAction(action);
    }
  };

  if (loading) return <p style={{ color: ayu.dim }}>Loading...</p>;
  if (error) return <p style={{ color: ayu.red }}>{error}</p>;
  if (!data) return <p style={{ color: ayu.dim }}>Not found.</p>;

  const { item, completion, stamp, branch, delta, actions } = data;

  return (
    <div style={{ maxWidth: '720px' }}>
      <button
        onClick={() => navigate(-1)}
        style={{
          background: 'transparent',
          border: 'none',
          color: ayu.dim,
          cursor: 'pointer',
          padding: 0,
          marginBottom: '12px',
          fontSize: '13px',
        }}
      >
        &larr; back
      </button>

      <div style={{ marginBottom: '16px' }}>
        <h2 style={{ color: ayu.fg, fontSize: '18px', fontWeight: 600, margin: 0 }}>
          {item.title}
        </h2>
        <div style={{ display: 'flex', gap: '8px', marginTop: '8px', alignItems: 'center' }}>
          <PriorityBadge priority={item.priority} />
          <StatusBadge status={item.status} />
          {item.type && <span style={{ color: ayu.dim, fontSize: '12px' }}>{item.type}</span>}
        </div>
      </div>

      {item.description && (
        <div
          style={{
            padding: '12px',
            background: ayu.surface,
            borderRadius: '6px',
            border: `1px solid ${ayu.border}`,
            marginBottom: '16px',
            whiteSpace: 'pre-wrap',
            color: ayu.fg,
            fontSize: '13px',
            lineHeight: 1.6,
          }}
        >
          {item.description}
        </div>
      )}

      <div
        style={{
          display: 'grid',
          gridTemplateColumns: '120px 1fr',
          gap: '6px 12px',
          fontSize: '13px',
          marginBottom: '16px',
        }}
      >
        <span style={{ color: ayu.dim }}>Posted by</span>
        <span style={{ color: ayu.fg }}>{item.posted_by || '-'}</span>
        <span style={{ color: ayu.dim }}>Claimed by</span>
        <span style={{ color: ayu.fg }}>{item.claimed_by || '-'}</span>
        <span style={{ color: ayu.dim }}>Effort</span>
        <span style={{ color: ayu.fg }}>{item.effort_level || '-'}</span>
        {item.tags && item.tags.length > 0 && (
          <>
            <span style={{ color: ayu.dim }}>Tags</span>
            <span style={{ color: ayu.fg }}>{item.tags.join(', ')}</span>
          </>
        )}
        {branch && (
          <>
            <span style={{ color: ayu.dim }}>Branch</span>
            <span style={{ color: ayu.purple }}>{branch}</span>
          </>
        )}
      </div>

      {completion && (
        <Section title="Completion">
          <div style={{ fontSize: '13px' }}>
            <p style={{ color: ayu.fg, margin: '0 0 4px' }}>
              Completed by: <span style={{ color: ayu.accent }}>{completion.completed_by}</span>
            </p>
            {completion.evidence && (
              <p style={{ color: ayu.fg, margin: '0 0 4px' }}>Evidence: {completion.evidence}</p>
            )}
            {completion.validated_by && (
              <p style={{ color: ayu.fg, margin: 0 }}>
                Validated by: <span style={{ color: ayu.green }}>{completion.validated_by}</span>
              </p>
            )}
          </div>
        </Section>
      )}

      {stamp && (
        <Section title="Stamp">
          <div style={{ fontSize: '13px', color: ayu.fg }}>
            <p style={{ margin: '0 0 4px' }}>
              Author: <span style={{ color: ayu.accent }}>{stamp.author}</span>
            </p>
            <p style={{ margin: '0 0 4px' }}>Subject: {stamp.subject}</p>
            <p style={{ margin: '0 0 4px' }}>
              Quality: {stamp.quality} / Reliability: {stamp.reliability}
            </p>
            {stamp.message && <p style={{ margin: 0 }}>{stamp.message}</p>}
          </div>
        </Section>
      )}

      {delta && (
        <Section title="Branch Delta">
          <pre
            style={{
              fontSize: '12px',
              color: ayu.fg,
              margin: 0,
              whiteSpace: 'pre-wrap',
              fontFamily: 'inherit',
            }}
          >
            {delta}
          </pre>
        </Section>
      )}

      {actions.length > 0 && (
        <div style={{ display: 'flex', gap: '8px', marginTop: '16px', flexWrap: 'wrap' }}>
          {actions.map((action) => (
            <ActionButton
              key={action}
              action={action}
              onAction={async () => onActionClick(action)}
            />
          ))}
        </div>
      )}

      {confirm && (
        <ConfirmDialog
          message={`Are you sure you want to ${confirm} this item?`}
          onCancel={() => setConfirm(null)}
          onConfirm={() => {
            const action = confirm;
            setConfirm(null);
            handleAction(action);
          }}
        />
      )}
    </div>
  );
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div
      style={{
        padding: '12px',
        background: ayu.surface,
        borderRadius: '6px',
        border: `1px solid ${ayu.border}`,
        marginBottom: '12px',
      }}
    >
      <h3
        style={{
          color: statusColor[title.toLowerCase()] || ayu.accent,
          fontSize: '13px',
          fontWeight: 600,
          margin: '0 0 8px',
        }}
      >
        {title}
      </h3>
      {children}
    </div>
  );
}
