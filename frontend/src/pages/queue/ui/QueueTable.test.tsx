import { MantineProvider } from '@mantine/core';
import { act, render, screen } from '@testing-library/react';
import type { CustomerRow } from '@entities/customer';
import { QueueTable } from './QueueTable';

const row = (id: number, expectedLossCents: number): CustomerRow => ({
  id,
  email: `c${id}@example.com`,
  balance: 1000,
  totalReplenished: 5000,
  score: 60,
  level: 'high',
  expectedLossCents,
  topReason: 'мало денег',
});

const table = (rows: CustomerRow[], version: number) => (
  <MantineProvider>
    <QueueTable
      rows={rows}
      version={version}
      asOf="2026-10-20T00:00:00Z"
      sort="expectedLoss"
      order="desc"
      onSort={() => {}}
      onOpen={() => {}}
    />
  </MantineProvider>
);

const changedIds = () =>
  screen
    .getAllByRole('row')
    .filter((r) => r.getAttribute('data-changed') === 'true')
    .map((r) => r.textContent);

describe('QueueTable: подсветка изменившихся строк', () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it('новая версия: подсвечена только изменившаяся строка и через 2 с снята', () => {
    const { rerender } = render(table([row(1, 100), row(2, 200)], 1));
    expect(changedIds()).toEqual([]);
    rerender(table([row(1, 100), row(2, 999)], 2));
    expect(changedIds()).toHaveLength(1);
    expect(changedIds()[0]).toContain('c2@example.com');
    act(() => {
      vi.advanceTimersByTime(2100);
    });
    expect(changedIds()).toEqual([]);
  });

  it('без новой версии подсветки нет', () => {
    const { rerender } = render(table([row(1, 100), row(2, 200)], 1));
    rerender(table([row(1, 100), row(2, 999)], 1));
    expect(changedIds()).toEqual([]);
  });
});

describe('QueueTable: выборка', () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it('в таблице нет колонки «Работа»', () => {
    render(table([row(1, 100)], 1));
    expect(screen.queryByRole('columnheader', { name: 'Работа' })).toBeNull();
  });

  it('другая выборка (фильтр): новые строки не подсвечиваются', () => {
    const { rerender } = render(table([row(1, 100)], 1));
    rerender(table([row(3, 300), row(4, 400)], 1));
    expect(changedIds()).toEqual([]);
  });
});

describe('QueueTable: потеря по причине', () => {
  it('в строке есть потеря по причине, колонка «Потеря» помечена сортировкой', () => {
    render(
      <MantineProvider>
        <QueueTable
          rows={[{ ...row(1, 500_00), reasonLossCents: 120_00 }]}
          version={1}
          asOf="2026-10-20T00:00:00Z"
          sort="reasonLoss"
          order="desc"
          onSort={() => {}}
          onOpen={() => {}}
        />
      </MantineProvider>,
    );
    expect(screen.getByText(/по причине:/)).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: /Потеря/ })).toHaveAttribute(
      'aria-sort',
      'descending',
    );
  });
});
