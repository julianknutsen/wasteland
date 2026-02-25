import { useEffect, useState, useCallback } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import {
  detail, claim, unclaim, reject, close, deleteItem,
  done, accept, submitPR, applyBranch, discardBranch, branchDiff,
} from '../api/client';
import type { DetailResponse } from '../api/types';
import { ayu, statusColor } from '../styles/theme';
import { StatusBadge } from './StatusBadge';
import { PriorityBadge } from './PriorityBadge';
import { ActionButton } from './ActionButton';
import { ConfirmDialog } from './ConfirmDialog';

const destructiveActions = new Set(['delete', 'close', 'reject', 'discard']);

export function DetailView() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [data, setData] = useState<DetailResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [confirm, setConfirm] = useState<string | null>(null);
  const [diffContent, setDiffContent] = useState<string | null>(null);
  const [diffLoading, setDiffLoading] = useState(false);
  const [evidenceInput, setEvidenceInput] = useState('');
  const [showDoneForm, setShowDoneForm] = useState(false);

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
    if (!id || !data) return;
    try {
      switch (action) {
        case 'claim': await claim(id); break;
        case 'unclaim': await unclaim(id); break;
        case 'reject': await reject(id); break;
        case 'close': await close(id); break;
        case 'delete':
          await deleteItem(id);
          navigate('/');
          return;
        case 'accept': await accept(id); break;
        case 'submit_pr':
          if (data.branch) {
            const resp = await submitPR(data.branch);
            setData({ ...data, pr_url: resp.url });
          }
          return;
        case 'apply':
          if (data.branch) {
            await applyBranch(data.branch);
          }
          break;
        case 'discard':
          if (data.branch) {
            await discardBranch(data.branch);
          }
          break;
        default: return;
      }
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : `Failed to ${action}`);
    }
  };

  const handleDone = async () => {
    if (!id || !evidenceInput.trim()) return;
    try {
      await done(id, evidenceInput.trim());
      setShowDoneForm(false);
      setEvidenceInput('');
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to submit');
    }
  };

  const handleLoadDiff = async () => {
    if (!data?.branch) return;
    setDiffLoading(true);
    try {
      const resp = await branchDiff(data.branch);
      setDiffContent(resp.diff);
    } catch (e) {
      setDiffContent(`Error loading diff: ${e instanceof Error ? e.message : 'unknown error'}`);
    } finally {
      setDiffLoading(false);
    }
  };

  const onActionClick = (action: string) => {
    if (action === 'done') {
      setShowDoneForm(true);
      return;
    }
    if (destructiveActions.has(action)) {
      setConfirm(action);
    } else {
      handleAction(action);
    }
  };

  if (loading) return <p style={{ color: ayu.dim }}>Loading...</p>;
  if (error) return <p style={{ color: ayu.red }}>{error}</p>;
  if (!data) return <p style={{ color: ayu.dim }}>Not found.</p>;

  const { item, completion, stamp, branch, main_status, pr_url, delta, actions, branch_actions } = data;

  // Branch actions are computed by the SDK (mode-aware: submit_pr/apply/discard).
  const branchActions = branch_actions || [];

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
        {branch && main_status && main_status !== item.status && (
          <>
            <span style={{ color: ayu.dim }}>Pending</span>
            <span style={{ color: ayu.accent }}>{main_status} &rarr; {item.status}</span>
          </>
        )}
        {branch && (
          <>
            <span style={{ color: ayu.dim }}>Branch</span>
            <span style={{ color: ayu.purple }}>{branch}</span>
          </>
        )}
        {pr_url && (
          <>
            <span style={{ color: ayu.dim }}>PR</span>
            <a
              href={pr_url}
              target="_blank"
              rel="noopener noreferrer"
              style={{ color: ayu.green, textDecoration: 'none' }}
            >
              {pr_url}
            </a>
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
          {branch && diffContent === null && (
            <button
              onClick={handleLoadDiff}
              disabled={diffLoading}
              style={{
                marginTop: '8px',
                padding: '4px 12px',
                background: 'transparent',
                border: `1px solid ${ayu.border}`,
                borderRadius: '4px',
                color: ayu.dim,
                cursor: diffLoading ? 'wait' : 'pointer',
                fontSize: '12px',
              }}
            >
              {diffLoading ? 'Loading diff...' : 'View diff'}
            </button>
          )}
          {diffContent && (
            <pre
              style={{
                marginTop: '8px',
                fontSize: '12px',
                color: ayu.fg,
                whiteSpace: 'pre-wrap',
                fontFamily: 'inherit',
              }}
            >
              {diffContent}
            </pre>
          )}
        </Section>
      )}

      {showDoneForm && (
        <Section title="Submit for Review">
          <div style={{ fontSize: '13px' }}>
            <label style={{ color: ayu.dim, display: 'block', marginBottom: '4px' }}>
              Evidence (URL or description)
            </label>
            <input
              type="text"
              value={evidenceInput}
              onChange={(e) => setEvidenceInput(e.target.value)}
              placeholder="https://github.com/..."
              style={{
                width: '100%',
                padding: '6px 8px',
                background: ayu.bg,
                border: `1px solid ${ayu.border}`,
                borderRadius: '4px',
                color: ayu.fg,
                fontSize: '13px',
                boxSizing: 'border-box',
              }}
              onKeyDown={(e) => { if (e.key === 'Enter') handleDone(); }}
            />
            <div style={{ display: 'flex', gap: '8px', marginTop: '8px' }}>
              <button
                onClick={handleDone}
                disabled={!evidenceInput.trim()}
                style={{
                  padding: '4px 12px',
                  background: 'transparent',
                  border: `1px solid ${ayu.green}`,
                  borderRadius: '4px',
                  color: ayu.green,
                  cursor: evidenceInput.trim() ? 'pointer' : 'not-allowed',
                  fontSize: '13px',
                  opacity: evidenceInput.trim() ? 1 : 0.5,
                }}
              >
                Submit
              </button>
              <button
                onClick={() => { setShowDoneForm(false); setEvidenceInput(''); }}
                style={{
                  padding: '4px 12px',
                  background: 'transparent',
                  border: `1px solid ${ayu.border}`,
                  borderRadius: '4px',
                  color: ayu.dim,
                  cursor: 'pointer',
                  fontSize: '13px',
                }}
              >
                Cancel
              </button>
            </div>
          </div>
        </Section>
      )}

      {(actions.length > 0 || branchActions.length > 0) && (
        <div style={{ display: 'flex', gap: '8px', marginTop: '16px', flexWrap: 'wrap' }}>
          {actions.map((action) => (
            <ActionButton
              key={action}
              action={action}
              onAction={async () => onActionClick(action)}
            />
          ))}
          {branchActions.map((action) => (
            <ActionButton
              key={action}
              action={action.replace('_', ' ')}
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
