import { MantineProvider } from '@mantine/core';
import { render, screen } from '@testing-library/react';
import { LevelBadge } from '../level-badge';
import { RescueWindow } from './index';

const wrap = (node: React.ReactNode) => render(<MantineProvider>{node}</MantineProvider>);

describe('RescueWindow', () => {
  it('окно закрывается: подпись «до удаления» и метка «сейчас»', () => {
    // срок вышел 2 д 17 ч назад → до удаления 2 д 07 ч
    wrap(<RescueWindow expiresAt="2026-10-17T00:00:00Z" asOf="2026-10-19T17:00:00Z" />);
    expect(screen.getByText(/до удаления 2 д 07 ч/)).toBeInTheDocument();
    expect(screen.getByText(/сейчас/)).toBeInTheDocument();
  });

  it('меньше суток — подпись всё равно текстом', () => {
    wrap(<RescueWindow expiresAt="2026-10-14T20:00:00Z" asOf="2026-10-19T08:00:00Z" />);
    expect(screen.getByText(/до удаления 12 ч/)).toBeInTheDocument();
  });

  it('окно закрыто', () => {
    wrap(<RescueWindow expiresAt="2026-10-01T00:00:00Z" asOf="2026-10-19T00:00:00Z" />);
    expect(screen.getByText('окно закрыто')).toBeInTheDocument();
    expect(screen.queryByText(/сейчас/)).toBeNull();
  });

  it('срок ещё не наступил', () => {
    wrap(<RescueWindow expiresAt="2026-10-21T00:00:00Z" asOf="2026-10-19T00:00:00Z" />);
    expect(screen.getByText(/до срока 2 д 00 ч/)).toBeInTheDocument();
  });
});

describe('LevelBadge', () => {
  it('уровень написан словом', () => {
    wrap(<LevelBadge level="high" />);
    expect(screen.getByText('Высокий риск')).toBeInTheDocument();
  });
});
