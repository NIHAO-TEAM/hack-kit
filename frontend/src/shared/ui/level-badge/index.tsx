import { Badge } from '@mantine/core';
import type { components } from '../../api';

export type RiskLevel = components['schemas']['RiskLevel'];

type Tone = { bg: string; c: string; bd: string };

const levels: Record<RiskLevel, { label: string; fw: number; tone: Tone }> = {
  high: {
    label: 'Высокий риск',
    fw: 600,
    tone: { bg: 'var(--critical-tint)', c: 'var(--critical-on-tint)', bd: 'var(--critical-fg)' },
  },
  medium: {
    label: 'Средний риск',
    fw: 600,
    tone: { bg: 'var(--warn-tint)', c: 'var(--warn-on-tint)', bd: 'var(--warn-fg)' },
  },
  low: {
    label: 'Низкий риск',
    fw: 500,
    tone: { bg: 'var(--surface-sunken)', c: 'var(--text-muted)', bd: 'var(--border)' },
  },
  unscored: {
    label: 'Без оценки',
    fw: 500,
    tone: { bg: 'var(--surface-sunken)', c: 'var(--text-muted)', bd: 'var(--border)' },
  },
  lost: {
    label: 'Потерян',
    fw: 500,
    tone: { bg: 'var(--lost-tint)', c: 'var(--lost-on-tint)', bd: 'var(--border)' },
  },
};

/**
 * Уровень риска словом и цветом. Красный — «Высокий риск», оранжевый — «Средний риск»
 * (ADR-0003, дополнение 25.10); подпись остаётся, цвет её дублирует. Низкий риск и «Без оценки»
 * нейтральные, серый — «Потерян». Цвет фона строк очереди это не меняет (запрет ADR-0002).
 */
export function LevelBadge({ level }: { level: RiskLevel }) {
  const { label, fw, tone } = levels[level];
  return (
    <Badge
      variant="light"
      radius="xs"
      tt="none"
      fw={fw}
      bg={tone.bg}
      c={tone.c}
      bd={`1px solid ${tone.bd}`}
      data-level={level}
    >
      {label}
    </Badge>
  );
}
