import type {
  BrowseFilter,
  BrowseResponse,
  ConfigResponse,
  DashboardResponse,
  DetailResponse,
  ErrorResponse,
  MutationResponse,
} from './types';

class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message);
    this.name = 'ApiError';
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const resp = await fetch(path, init);
  const body = await resp.json();
  if (!resp.ok) {
    throw new ApiError(resp.status, (body as ErrorResponse).error || resp.statusText);
  }
  return body as T;
}

function buildQuery(filter: BrowseFilter): string {
  const params = new URLSearchParams();
  if (filter.status) params.set('status', filter.status);
  if (filter.type) params.set('type', filter.type);
  if (filter.priority !== undefined && filter.priority >= 0) params.set('priority', String(filter.priority));
  if (filter.project) params.set('project', filter.project);
  if (filter.search) params.set('search', filter.search);
  if (filter.sort) params.set('sort', filter.sort);
  if (filter.limit) params.set('limit', String(filter.limit));
  const qs = params.toString();
  return qs ? `?${qs}` : '';
}

export async function browse(filter: BrowseFilter = {}): Promise<BrowseResponse> {
  return request<BrowseResponse>(`/api/wanted${buildQuery(filter)}`);
}

export async function detail(id: string): Promise<DetailResponse> {
  return request<DetailResponse>(`/api/wanted/${id}`);
}

export async function dashboard(): Promise<DashboardResponse> {
  return request<DashboardResponse>('/api/dashboard');
}

export async function config(): Promise<ConfigResponse> {
  return request<ConfigResponse>('/api/config');
}

export async function claim(id: string): Promise<MutationResponse> {
  return request<MutationResponse>(`/api/wanted/${id}/claim`, { method: 'POST' });
}

export async function unclaim(id: string): Promise<MutationResponse> {
  return request<MutationResponse>(`/api/wanted/${id}/unclaim`, { method: 'POST' });
}

export async function reject(id: string, reason?: string): Promise<MutationResponse> {
  return request<MutationResponse>(`/api/wanted/${id}/reject`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ reason: reason || '' }),
  });
}

export async function close(id: string): Promise<MutationResponse> {
  return request<MutationResponse>(`/api/wanted/${id}/close`, { method: 'POST' });
}

export async function deleteItem(id: string): Promise<MutationResponse> {
  return request<MutationResponse>(`/api/wanted/${id}`, { method: 'DELETE' });
}

export async function sync(): Promise<void> {
  await request<Record<string, string>>('/api/sync', { method: 'POST' });
}

export { ApiError };
