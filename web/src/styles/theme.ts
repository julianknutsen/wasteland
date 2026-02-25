export const ayu = {
  bg: '#e8dcc8',
  bgAlt: '#d9c9b0',
  bgDark: '#3e2723',
  fg: '#2c1810',
  fgMuted: '#5d4037',
  fgLight: '#f5e6d3',
  accent: '#8b2500',
  accentHover: '#a0522d',
  accentLight: '#d4a574',
  green: '#2e7d32',
  red: '#8b2500',
  blue: '#4a6741',
  orange: '#cd7f32',
  purple: '#6a1b9a',
  cyan: '#00695c',
  dim: '#5d4037',
  surface: '#f5e6d3',
  surfaceDark: '#4a3728',
  border: '#8b7355',
  borderDark: '#5d4037',
  brass: '#cd7f32',
  bronze: '#8b4513',
  copper: '#b87333',
  steel: '#70798c',
};

export const statusColor: Record<string, string> = {
  open: ayu.green,
  claimed: ayu.steel,
  in_review: ayu.brass,
  completed: ayu.accent,
  withdrawn: ayu.dim,
};

export const priorityLabel = (p: number): string => {
  if (p < 0) return 'all';
  return `P${p}`;
};
