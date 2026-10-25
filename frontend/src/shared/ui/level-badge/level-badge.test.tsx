import { MantineProvider } from '@mantine/core';
import { render, screen } from '@testing-library/react';
import { LevelBadge, type RiskLevel } from './index';

const cases: [RiskLevel, string, string][] = [
  ['high', 'Высокий риск', 'critical-tint'],
  ['medium', 'Средний риск', 'warn-tint'],
  ['low', 'Низкий риск', 'surface-sunken'],
  ['unscored', 'Без оценки', 'surface-sunken'],
  ['lost', 'Потерян', 'lost-tint'],
];

describe('LevelBadge', () => {
  it.each(cases)('%s: подпись словами и токен фона', (level, label, token) => {
    render(
      <MantineProvider>
        <LevelBadge level={level} />
      </MantineProvider>,
    );
    const badge = screen.getByText(label).closest('[data-level]') as HTMLElement;
    expect(badge).toHaveAttribute('data-level', level);
    expect(badge.getAttribute('style')).toContain(`var(--${token})`);
  });
});
