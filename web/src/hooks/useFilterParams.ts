import { useSearchParams } from 'react-router-dom';
import { useCallback, useMemo } from 'react';
import type { BrowseFilter } from '../api/types';

export function useFilterParams(): [BrowseFilter, (filter: BrowseFilter) => void] {
  const [searchParams, setSearchParams] = useSearchParams();

  const filter = useMemo<BrowseFilter>(() => {
    const f: BrowseFilter = {};
    const status = searchParams.get('status');
    const type = searchParams.get('type');
    const sort = searchParams.get('sort');
    const search = searchParams.get('search');
    const priority = searchParams.get('priority');
    if (status) f.status = status;
    if (type) f.type = type;
    if (sort) f.sort = sort;
    if (search) f.search = search;
    if (priority) f.priority = Number(priority);
    return f;
  }, [searchParams]);

  const setFilter = useCallback((f: BrowseFilter) => {
    const params = new URLSearchParams();
    if (f.status) params.set('status', f.status);
    if (f.type) params.set('type', f.type);
    if (f.sort && f.sort !== 'priority') params.set('sort', f.sort);
    if (f.search) params.set('search', f.search);
    if (f.priority !== undefined && f.priority >= 0) params.set('priority', String(f.priority));
    setSearchParams(params, { replace: true });
  }, [setSearchParams]);

  return [filter, setFilter];
}
