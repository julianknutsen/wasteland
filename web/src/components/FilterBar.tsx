import { ayu } from '../styles/theme';
import type { BrowseFilter } from '../api/types';

const statuses = ['', 'open', 'claimed', 'in_review', 'completed'];
const types = ['', 'feature', 'bug', 'design', 'rfc', 'docs'];
const sorts = ['priority', 'newest', 'alpha'];

interface FilterBarProps {
  filter: BrowseFilter;
  onChange: (filter: BrowseFilter) => void;
}

const selectStyle: React.CSSProperties = {
  padding: '6px 10px',
  borderRadius: '4px',
  border: `1px solid ${ayu.border}`,
  background: ayu.surface,
  color: ayu.fg,
  fontSize: '14px',
  fontFamily: "'Crimson Text', Georgia, serif",
};

const inputStyle: React.CSSProperties = {
  ...selectStyle,
  width: '200px',
};

export function FilterBar({ filter, onChange }: FilterBarProps) {
  return (
    <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap', alignItems: 'center' }}>
      <select
        value={filter.status || ''}
        onChange={(e) => onChange({ ...filter, status: e.target.value || undefined })}
        style={selectStyle}
      >
        {statuses.map((s) => (
          <option key={s} value={s}>
            {s || 'all statuses'}
          </option>
        ))}
      </select>

      <select
        value={filter.type || ''}
        onChange={(e) => onChange({ ...filter, type: e.target.value || undefined })}
        style={selectStyle}
      >
        {types.map((t) => (
          <option key={t} value={t}>
            {t || 'all types'}
          </option>
        ))}
      </select>

      <select
        value={filter.sort || 'priority'}
        onChange={(e) => onChange({ ...filter, sort: e.target.value })}
        style={selectStyle}
      >
        {sorts.map((s) => (
          <option key={s} value={s}>
            {s}
          </option>
        ))}
      </select>

      <input
        type="text"
        placeholder="search..."
        value={filter.search || ''}
        onChange={(e) => onChange({ ...filter, search: e.target.value || undefined })}
        style={inputStyle}
      />
    </div>
  );
}
