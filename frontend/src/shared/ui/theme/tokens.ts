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
    bg: '#000000',
    surface: '#0f1114',
    'surface-sunken': '#08090b',
    'surface-hover': '#1c2026',
    'changed-tint': '#10303c',
    border: '#262a31',
    'border-strong': '#3a404a',
    'field-border': '#6b7380',
    'text-strong': '#f4f5f7',
    text: '#d7dbe2',
    'text-muted': '#a1a8b4',
    'text-subtle': '#8d95a2',
    'focus-ring': '#4cc6f1',
    primary: '#4cc6f1',
    'primary-hover': '#72d4f5',
    'on-primary': '#000000',
    'primary-fg': '#4cc6f1',
    'warn-fg': '#e97c35',
    'warn-tint': '#2e2012',
    'warn-on-tint': '#fdba74',
    'warn-solid': '#e97c35',
    'critical-fg': '#ff6b6b',
    'critical-tint': '#341717',
    'critical-on-tint': '#ff9a9a',
    'critical-solid': '#ff6b6b',
    'effect-fg': '#4ade80',
    'effect-tint': '#0f2a1a',
    'effect-on-tint': '#86efac',
    'effect-solid': '#4ade80',
    'lost-fg': '#a1a8b4',
    'lost-tint': '#1f2329',
    'lost-on-tint': '#b4bac4',
    'lost-solid': '#6b7380',
  },
  light: {
    bg: '#f4f4f4',
    surface: '#ffffff',
    'surface-sunken': '#eef0f3',
    'surface-hover': '#f0f2f5',
    'changed-tint': '#eaf7fc',
    border: '#e1e4e9',
    'border-strong': '#c5cad2',
    'field-border': '#737b88',
    'text-strong': '#0f1216',
    text: '#2a2f38',
    'text-muted': '#555d6b',
    'text-subtle': '#5f6877',
    'focus-ring': '#08708f',
    primary: '#08708f',
    'primary-hover': '#06566f',
    'on-primary': '#ffffff',
    'primary-fg': '#08708f',
    'warn-fg': '#c2410c',
    'warn-tint': '#fff0e0',
    'warn-on-tint': '#9a3412',
    'warn-solid': '#c2410c',
    'critical-fg': '#b91c1c',
    'critical-tint': '#fde8e8',
    'critical-on-tint': '#991b1b',
    'critical-solid': '#b91c1c',
    'effect-fg': '#117a37',
    'effect-tint': '#dcf5e4',
    'effect-on-tint': '#14532d',
    'effect-solid': '#117a37',
    'lost-fg': '#555d6b',
    'lost-tint': '#eceef1',
    'lost-on-tint': '#454c58',
    'lost-solid': '#737b88',
  },
};

export const tokenNames = Object.keys(colorTokens.dark) as TokenName[];
