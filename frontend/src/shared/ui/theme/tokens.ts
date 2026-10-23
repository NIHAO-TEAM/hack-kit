/** Значения токенов ADR-0002 для обеих тем. Имена — по роли, не по оттенку. */
export type TokenName =
  | 'bg'
  | 'surface'
  | 'surface-sunken'
  | 'surface-hover'
  | 'changed-tint'
  | 'border'
  | 'border-strong'
  | 'field-border'
  | 'text-strong'
  | 'text'
  | 'text-muted'
  | 'text-subtle'
  | 'focus-ring'
  | 'primary'
  | 'primary-hover'
  | 'on-primary'
  | 'primary-fg'
  | 'warn-fg'
  | 'warn-tint'
  | 'warn-on-tint'
  | 'warn-solid'
  | 'critical-fg'
  | 'critical-tint'
  | 'critical-on-tint'
  | 'critical-solid'
  | 'effect-fg'
  | 'effect-tint'
  | 'effect-on-tint'
  | 'effect-solid'
  | 'lost-fg'
  | 'lost-tint'
  | 'lost-on-tint'
  | 'lost-solid';

export type ColorMode = 'light' | 'dark';

export const colorTokens: Record<ColorMode, Record<TokenName, string>> = {
  dark: {
    bg: '#0d0f12',
    surface: '#15181d',
    'surface-sunken': '#101317',
    'surface-hover': '#1c2026',
    'changed-tint': '#2a2350',
    border: '#262a31',
    'border-strong': '#3a404a',
    'field-border': '#6b7380',
    'text-strong': '#f4f5f7',
    text: '#d7dbe2',
    'text-muted': '#a1a8b4',
    'text-subtle': '#8d95a2',
    'focus-ring': '#a99bff',
    primary: '#6d4af0',
    'primary-hover': '#7553f5',
    'on-primary': '#ffffff',
    'primary-fg': '#a99bff',
    'warn-fg': '#fb923c',
    'warn-tint': '#2e2012',
    'warn-on-tint': '#fdba74',
    'warn-solid': '#fb923c',
    'critical-fg': '#ff6b6b',
    'critical-tint': '#341717',
    'critical-on-tint': '#ff9a9a',
    'critical-solid': '#ff6b6b',
    'effect-fg': '#2dd4bf',
    'effect-tint': '#0f2a27',
    'effect-on-tint': '#5eead4',
    'effect-solid': '#2dd4bf',
    'lost-fg': '#a1a8b4',
    'lost-tint': '#1f2329',
    'lost-on-tint': '#b4bac4',
    'lost-solid': '#6b7380',
  },
  light: {
    bg: '#f5f6f8',
    surface: '#ffffff',
    'surface-sunken': '#eef0f3',
    'surface-hover': '#f0f2f5',
    'changed-tint': '#f1edfe',
    border: '#e1e4e9',
    'border-strong': '#c5cad2',
    'field-border': '#737b88',
    'text-strong': '#0f1216',
    text: '#2a2f38',
    'text-muted': '#555d6b',
    'text-subtle': '#5f6877',
    'focus-ring': '#5b3fe0',
    primary: '#5b3fe0',
    'primary-hover': '#4b30c9',
    'on-primary': '#ffffff',
    'primary-fg': '#5b3fe0',
    'warn-fg': '#c2410c',
    'warn-tint': '#fff0e0',
    'warn-on-tint': '#9a3412',
    'warn-solid': '#c2410c',
    'critical-fg': '#b91c1c',
    'critical-tint': '#fde8e8',
    'critical-on-tint': '#991b1b',
    'critical-solid': '#b91c1c',
    'effect-fg': '#0f766e',
    'effect-tint': '#dcf5f1',
    'effect-on-tint': '#115e59',
    'effect-solid': '#0f766e',
    'lost-fg': '#555d6b',
    'lost-tint': '#eceef1',
    'lost-on-tint': '#454c58',
    'lost-solid': '#737b88',
  },
};

export const tokenNames = Object.keys(colorTokens.dark) as TokenName[];
