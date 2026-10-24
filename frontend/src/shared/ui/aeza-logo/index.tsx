import { Box } from '@mantine/core';
import { AezaMark } from '../aeza-mark';
import logoUrl from './aeza-logo.svg';

// Пропорции файла: viewBox 354 × 85.
const RATIO = 354 / 85;

/**
 * Официальный логотип aéza (ADR-0004): файл с aeza.net/logo.svg без правок.
 * Цвет не меняем: файл рассчитан на тёмный фон, плашку под него задаёт место, где логотип стоит.
 * Охранное поле вокруг задаёт то же место.
 */
export function AezaLogo({ height }: { height: number }) {
  return (
    <img
      src={logoUrl}
      alt="aéza"
      width={Math.round(height * RATIO)}
      height={height}
      style={{ display: 'block', flexShrink: 0 }}
    />
  );
}

/**
 * Логотип на тёмной плашке (цвет файла не меняем, в светлой теме плашка не даёт пропасть белому).
 * На узком экране вместо логотипа значок. Отступ плашки — охранное поле.
 */
export function AezaBrand({ height = 28 }: { height?: number }) {
  return (
    <>
      <Box
        visibleFrom="sm"
        bg="dark.9"
        px="md"
        py={6}
        style={{ borderRadius: 'var(--mantine-radius-sm)', flexShrink: 0 }}
      >
        <AezaLogo height={height} />
      </Box>
      <Box hiddenFrom="sm" style={{ flexShrink: 0 }}>
        <AezaMark size={height} />
      </Box>
    </>
  );
}
