export const ayu = {
  bg: '#0D1017',
  fg: '#BFBDB6',
  accent: '#E6B450',
  green: '#7FD962',
  red: '#D95757',
  blue: '#39BAE6',
  orange: '#FF8F40',
  purple: '#D2A6FF',
  cyan: '#95E6CB',
  dim: '#565B66',
  surface: '#131721',
  border: '#1D2433',
};

export const statusColor: Record<string, string> = {
  open: ayu.green,
  claimed: ayu.blue,
  in_review: ayu.orange,
  completed: ayu.accent,
  withdrawn: ayu.dim,
};

export const priorityLabel = (p: number): string => {
  if (p < 0) return 'all';
  return `P${p}`;
};
