import { Badge } from '@mantine/core';
import type { components } from '../../api';

export type RiskLevel = components['schemas']['RiskLevel'];

const levels: Record<RiskLevel, { label: string; tone: 'warn' | 'critical' | 'lost' | null }> = {
  high: { label: 'Высокий риск', tone: 'critical' },
  medium: { label: 'Средний риск', tone: 'warn' },
  low: { label: 'Низкий риск', tone: null },
  unscored: { label: 'Без оценки', tone: null },
  lost: { label: 'Потерян', tone: 'lost' },
};

/** Уровень риска словом; цвет только дублирует. Низкий и без оценки — без цвета. */
export function LevelBadge({ level }: { level: RiskLevel }) {
  const { label, tone } = levels[level];
  return (
    <Badge
      variant="light"
      radius="xs"
      tt="none"
      fw={500}
      bg={tone ? `var(--${tone}-tint)` : 'var(--surface-sunken)'}
      c={tone ? `var(--${tone}-on-tint)` : 'var(--text-muted)'}
      bd={tone ? undefined : '1px solid var(--border)'}
    >
      {label}
    </Badge>
  );
}
