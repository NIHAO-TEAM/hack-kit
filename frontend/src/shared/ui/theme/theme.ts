import { createTheme, type CSSVariablesResolver, type MantineColorsTuple } from '@mantine/core';
import { colorTokens } from './tokens';

export const fontFamily =
  "'Onest', system-ui, -apple-system, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif";

const brand: MantineColorsTuple = [
  '#f1edff',
  '#e1d9ff',
  '#c4b5ff',
  '#a99bff',
  '#8d78ff',
  '#7553f5',
  '#5b3fe0',
  '#6d4af0',
  '#4b30c9',
  '#3a24a3',
];

export const theme = createTheme({
  fontFamily,
  headings: { fontFamily, fontWeight: '600' },
  primaryColor: 'brand',
  primaryShade: { light: 6, dark: 7 },
  colors: { brand },
  defaultRadius: 'md',
  radius: { xs: '4px', sm: '4px', md: '6px', lg: '12px', xl: '14px' },
  spacing: { xs: '4px', sm: '8px', md: '12px', lg: '16px', xl: '24px' },
  black: colorTokens.light['text-strong'],
  components: {
    Button: { defaultProps: { size: 'sm' } },
  },
});

const toVars = (mode: 'light' | 'dark') =>
  Object.fromEntries(
    Object.entries(colorTokens[mode]).map(([name, value]) => [`--${name}`, value]),
  );

/** Токены становятся CSS-переменными обеих тем; Mantine берёт из них поверхности и текст. */
export const cssVariablesResolver: CSSVariablesResolver = () => ({
  variables: {},
  light: {
    ...toVars('light'),
    '--mantine-color-body': colorTokens.light.bg,
    '--mantine-color-text': colorTokens.light.text,
    '--mantine-color-dimmed': colorTokens.light['text-muted'],
    '--mantine-color-default-border': colorTokens.light['field-border'],
  },
  dark: {
    ...toVars('dark'),
    '--mantine-color-body': colorTokens.dark.bg,
    '--mantine-color-text': colorTokens.dark.text,
    '--mantine-color-dimmed': colorTokens.dark['text-muted'],
    '--mantine-color-default-border': colorTokens.dark['field-border'],
    '--mantine-color-dark-6': colorTokens.dark.surface,
  },
});
