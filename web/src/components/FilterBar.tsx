import type { RefObject } from "react";
import type { BrowseFilter } from "../api/types";
import styles from "./FilterBar.module.css";

const statuses = ["", "open", "claimed", "in_review", "completed"];
const types = ["", "feature", "bug", "design", "rfc", "docs"];
const sorts = ["priority", "newest", "alpha"];

interface FilterBarProps {
  filter: BrowseFilter;
  onChange: (filter: BrowseFilter) => void;
  searchRef?: RefObject<HTMLInputElement | null>;
}

export function FilterBar({ filter, onChange, searchRef }: FilterBarProps) {
  return (
    <div className={styles.bar} role="search" aria-label="Filter wanted items">
      <select
        className={styles.select}
        aria-label="Filter by status"
        value={filter.status || ""}
        onChange={(e) => onChange({ ...filter, status: e.target.value || undefined })}
      >
        {statuses.map((s) => (
          <option key={s} value={s}>
            {s || "all statuses"}
          </option>
        ))}
      </select>

      <select
        className={styles.select}
        aria-label="Filter by type"
        value={filter.type || ""}
        onChange={(e) => onChange({ ...filter, type: e.target.value || undefined })}
      >
        {types.map((t) => (
          <option key={t} value={t}>
            {t || "all types"}
          </option>
        ))}
      </select>

      <select
        className={styles.select}
        aria-label="Sort order"
        value={filter.sort || "priority"}
        onChange={(e) => onChange({ ...filter, sort: e.target.value })}
      >
        {sorts.map((s) => (
          <option key={s} value={s}>
            {s}
          </option>
        ))}
      </select>

      <input
        ref={searchRef}
        className={styles.input}
        aria-label="Search items"
        type="text"
        placeholder="search..."
        value={filter.search || ""}
        onChange={(e) => onChange({ ...filter, search: e.target.value || undefined })}
      />
    </div>
  );
}
