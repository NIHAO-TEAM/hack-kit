import { Text } from '@mantine/core';
import { useMemo, type MouseEvent, type ReactNode } from 'react';
import type { CustomerOrder, CustomerRow, CustomerSort } from '@entities/customer';
import { formatMoney, useChangedRows } from '@shared/lib';
import {
  IconArrowDown,
  IconArrowUp,
  IconArrowsSort,
  LevelBadge,
  Money,
  RescueWindow,
} from '@shared/ui';
import classes from './QueueTable.module.css';

interface Column {
  label: string;
  sort?: CustomerSort;
  num?: boolean;
  /** Ячейка не сжимается: плашка уровня показывается целиком. */
  wide?: boolean;
}

const columns: Column[] = [
  { label: 'ID', sort: 'id' },
  { label: 'Email', sort: 'email' },
  { label: 'Баланс', sort: 'balance', num: true },
  { label: 'Пополнено', sort: 'totalReplenished', num: true },
  { label: 'Скор', sort: 'score', wide: true },
  { label: 'Потеря', sort: 'expectedLoss', num: true },
  { label: 'Причина' },
  { label: 'Окно спасения' },
  { label: 'Действие' },
];

function SortHeader({
  column,
  sort,
  order,
  onSort,
}: {
  column: Column;
  sort: CustomerSort;
  order: CustomerOrder;
  onSort: (column: CustomerSort) => void;
}) {
  if (!column.sort) return <>{column.label}</>;
  const active = sort === column.sort;
  const Icon = !active ? IconArrowsSort : order === 'asc' ? IconArrowUp : IconArrowDown;
  const target = column.sort;
  return (
    <button
      type="button"
      className={classes.sort}
      data-active={active}
      onClick={() => onSort(target)}
    >
      {column.label}
      <Icon size={14} aria-hidden />
    </button>
  );
}

function Score({ row }: { row: CustomerRow }) {
  return (
    <span className={classes.score}>
      {row.score != null ? (
        <Text span fw={600}>
          {Math.round(row.score)}{' '}
        </Text>
      ) : null}
      <LevelBadge level={row.level} />
    </span>
  );
}

function ActionCell({ row }: { row: CustomerRow }) {
  if (row.billing) return <span className={classes.billing}>к биллингу</span>;
  if (!row.topAction) return <span className={classes.billing}>—</span>;
  return (
    <span className={classes.action}>
      <span>{row.topAction.title}</span>
      <Money cents={row.topAction.effectCents} tone="effect" signed />
    </span>
  );
}

/** Всё, что видно в строке: изменилось любое поле — строка подсветится. */
const signature = (row: CustomerRow) =>
  JSON.stringify([
    row.balance,
    row.totalReplenished,
    row.score ?? null,
    row.level,
    row.expectedLossCents,
    row.topReason ?? null,
    row.topAction?.id ?? null,
    row.topAction?.effectCents ?? null,
    row.billing ?? false,
  ]);

/**
 * Рейтинг клиентов. Порядок строк даёт сервер: клик по заголовку только меняет `sort`/`order`.
 * На планшете и телефоне строки становятся карточками (CSS), разметка одна.
 */
export function QueueTable({
  rows,
  version,
  asOf,
  sort,
  order,
  onSort,
  onOpen,
}: {
  rows: CustomerRow[];
  /** Версия пересчёта, по которому построена страница: подсветка только при её смене. */
  version: number | null;
  asOf: string | null;
  sort: CustomerSort;
  order: CustomerOrder;
  onSort: (column: CustomerSort) => void;
  onOpen: (id: number) => void;
}) {
  const items = useMemo(() => rows.map((r) => ({ id: r.id, signature: signature(r) })), [rows]);
  const changed = useChangedRows(items, version);
  const ariaSort = (c: Column) =>
    c.sort && c.sort === sort ? (order === 'asc' ? 'ascending' : 'descending') : undefined;
  return (
    <table className={classes.table} aria-label="Клиенты под риском оттока">
      <thead>
        <tr>
          {columns.map((c) => (
            <th
              key={c.label}
              scope="col"
              aria-sort={ariaSort(c)}
              className={c.num ? classes.num : undefined}
            >
              <SortHeader column={c} sort={sort} order={order} onSort={onSort} />
            </th>
          ))}
        </tr>
      </thead>
      <tbody>
        {rows.map((row) => {
          const open = (e: MouseEvent) => {
            if (e.defaultPrevented) return;
            e.preventDefault();
            onOpen(row.id);
          };
          const dim = row.level === 'unscored' || row.level === 'lost';
          const cell = (i: number, content: ReactNode) => (
            <td
              data-label={columns[i]!.label}
              className={
                [columns[i]!.num && classes.num, columns[i]!.wide && classes.wide]
                  .filter(Boolean)
                  .join(' ') || undefined
              }
            >
              {content}
            </td>
          );
          return (
            <tr
              key={row.id}
              className={classes.row}
              data-dim={dim}
              data-changed={changed.has(row.id)}
              onClick={open}
            >
              {cell(0, row.id)}
              {cell(
                1,
                <a className={classes.email} href={`/queue/${row.id}`} onClick={open}>
                  {row.email}
                </a>,
              )}
              {cell(2, formatMoney(row.balance))}
              {cell(3, formatMoney(row.totalReplenished))}
              {cell(4, <Score row={row} />)}
              {cell(5, dim ? '—' : <Money cents={row.expectedLossCents} />)}
              {cell(6, <span className={classes.reason}>{row.topReason ?? '—'}</span>)}
              {cell(7, asOf ? <RescueWindow expiresAt={row.expiresAt ?? null} asOf={asOf} /> : '—')}
              {cell(8, <ActionCell row={row} />)}
            </tr>
          );
        })}
      </tbody>
    </table>
  );
}
