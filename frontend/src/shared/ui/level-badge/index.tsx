import { Badge } from '@mantine/core';
import type { components } from '../../api';

export type RiskLevel = components['schemas']['RiskLevel'];

const levels: Record<RiskLevel, { label: string; emphasis: boolean; lost?: boolean }> = {
  high: { label: 'Высокий риск', emphasis: true },
  medium: { label: 'Средний риск', emphasis: true },
  low: { label: 'Низкий риск', emphasis: false },
  unscored: { label: 'Без оценки', emphasis: false },
  lost: { label: 'Потерян', emphasis: false, lost: true },
};

/**
 * Уровень риска словом. Красный и оранжевый ADR-0002 привязывает ко времени до удаления
 * (RescueWindow), поэтому высокий и средний риск — нейтральная плашка, выше по весу: тёмный
 * текст и сильная рамка. Серый — только у «Потерян».
 */
export function LevelBadge({ level }: { level: RiskLevel }) {
  const { label, emphasis, lost } = levels[level];
  return (
    <Badge
      variant="light"
      radius="xs"
      tt="none"
      fw={emphasis ? 600 : 500}
      bg={lost ? 'var(--lost-tint)' : 'var(--surface-sunken)'}
      c={lost ? 'var(--lost-on-tint)' : emphasis ? 'var(--text-strong)' : 'var(--text-muted)'}
      bd={`1px solid ${emphasis ? 'var(--border-strong)' : 'var(--border)'}`}
    >
      {label}
    </Badge>
  );
}
