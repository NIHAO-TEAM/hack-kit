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

describe('QueueTable: колонка «Работа»', () => {
  it('у клиента в работе — статус, ответственный и срок; без задачи пусто', () => {
    const withTask: CustomerRow = {
      ...row(9, 500),
      task: {
        id: 1,
        status: 'in_work',
        assignee: 'marketer',
        dueAt: '2026-10-20T15:00:00Z',
        overdue: false,
      },
    };
    render(table([withTask, row(10, 100)], 1));
    const [, r9, r10] = screen.getAllByRole('row');
    expect(r9!.textContent).toContain('В работе');
    expect(r9!.textContent).toContain('marketer');
    expect(r9!.textContent).toContain('осталось 15 ч');
    expect(r10!.querySelector('td[data-label="Работа"]')!.textContent).toBe('');
  });

  it('просроченный срок — текстом', () => {
    const overdue: CustomerRow = {
      ...row(9, 500),
      task: {
        id: 1,
        status: 'contacted',
        assignee: 'jury',
        dueAt: '2026-10-19T00:00:00Z',
        overdue: true,
      },
    };
    render(table([overdue], 1));
    expect(screen.getByText(/срок прошёл/)).toBeInTheDocument();
    expect(screen.getByText('Связались')).toBeInTheDocument();
  });
});

describe('QueueTable: смена задачи подсвечивает строку', () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  const brief = (assignee: string) => ({
    id: 1,
    status: 'in_work' as const,
    assignee,
    dueAt: '2026-10-21T15:00:00Z',
    overdue: false,
  });

  it('взяли на другом экране: версия пересчёта та же, строка подсвечена и через 2 с снята', () => {
    const { rerender } = render(table([row(1, 100), row(2, 200)], 1));
    rerender(table([row(1, 100), { ...row(2, 200), task: brief('jury') }], 1));
    expect(changedIds()).toHaveLength(1);
    expect(changedIds()[0]).toContain('c2@example.com');
    act(() => {
      vi.advanceTimersByTime(2100);
    });
    expect(changedIds()).toEqual([]);
  });

  it('другая выборка (фильтр): новые строки не подсвечиваются', () => {
    const { rerender } = render(table([row(1, 100)], 1));
    rerender(table([row(3, 300), row(4, 400)], 1));
    expect(changedIds()).toEqual([]);
  });
});
