import { Button, Group, Select, Stack, TextInput, Title } from '@mantine/core';
import { useDebouncedValue } from '@mantine/hooks';
import { keepPreviousData, useQuery } from '@tanstack/react-query';
import { useRouter } from '@tanstack/react-router';
import { useEffect, useState } from 'react';
import {
  customersOptions,
  type CustomerParams,
  type CustomerSort,
  type RiskLevel,
} from '@entities/customer';
import { ApiError } from '@shared/api';
import type { Role } from '@entities/session';
import {
  IconSearch,
  StateEmpty,
  StateError,
  StateForbidden,
  StateLoading,
  StateOffline,
} from '@shared/ui';
import { summaryOptions } from '@entities/summary';
import { DeletionStrip } from './DeletionStrip';
import { QueueTable } from './QueueTable';
import { SummaryCards } from './SummaryCards';
import { WhatIfPanel } from './WhatIfPanel';

const levelOptions: Array<{ value: RiskLevel; label: string }> = [
  { value: 'high', label: 'Высокий риск' },
  { value: 'medium', label: 'Средний риск' },
  { value: 'low', label: 'Низкий риск' },
  { value: 'unscored', label: 'Без оценки' },
  { value: 'lost', label: 'Потерян' },
];

/** Первый клик по заголовку: числа — от больших к меньшим, текст и ID — по возрастанию. */
const firstOrder: Record<CustomerSort, 'asc' | 'desc'> = {
  id: 'asc',
  email: 'asc',
  balance: 'desc',
  totalReplenished: 'desc',
  score: 'desc',
  expectedLoss: 'desc',
};

export function QueuePage({
  params,
  onParams,
  role,
}: {
  /** Сортировка и фильтры живут в адресе: ссылка на очередь воспроизводит вид. */
  params: CustomerParams;
  onParams: (next: CustomerParams) => void;
  role: Role;
}) {
  const router = useRouter();
  const customers = useQuery({ ...customersOptions(params), placeholderData: keepPreviousData });
  const summary = useQuery(summaryOptions);
  // Полоса «удалит биллинг» не зависит от поиска и фильтра: к биллингу уходят все.
  const unfiltered = useQuery(customersOptions({ sort: 'expectedLoss', order: 'desc' }));

  const [whatIf, setWhatIf] = useState(false);
  const [search, setSearch] = useState(params.q ?? '');
  const [debounced] = useDebouncedValue(search, 300);
  useEffect(() => {
    if (debounced !== (params.q ?? '')) onParams({ ...params, q: debounced || undefined });
    // eslint-disable-next-line react-hooks/exhaustive-deps -- реагируем только на ввод
  }, [debounced]);

  const open = (id: number) => void router.navigate({ href: `/queue/${id}` });
  const onSort = (column: CustomerSort) =>
    onParams({
      ...params,
      sort: column,
      order:
        params.sort === column ? (params.order === 'asc' ? 'desc' : 'asc') : firstOrder[column],
    });

  const error = customers.error ?? summary.error;
  // Время — время стенда из сводки; часы браузера не подставляем.
  const asOf = summary.data?.asOf ?? null;

  let body;
  if (error instanceof ApiError && error.status === 403) {
    body = <StateForbidden>Очередь доступна маркетологу и администратору.</StateForbidden>;
  } else if (customers.isPending) {
    body = <StateLoading label="Загрузка очереди" />;
  } else if (customers.isError && !customers.data) {
    body =
      error instanceof ApiError && error.isNetwork ? (
        <StateOffline onRetry={() => void customers.refetch()} />
      ) : (
        <StateError
          title="Не удалось загрузить очередь"
          requestId={error instanceof ApiError ? error.requestId : null}
          onRetry={() => void customers.refetch()}
        >
          Сервер ответил ошибкой. Повторите попытку.
        </StateError>
      );
  } else if (customers.data && customers.data.items.length === 0) {
    body =
      params.q || params.level ? (
        <StateEmpty title="Никого не нашли">Измените поиск или уровень риска.</StateEmpty>
      ) : (
        <StateEmpty title="Сегодня некого спасать">
          Клиентов под риском нет. Очередь обновится сама, когда данные изменятся.
        </StateEmpty>
      );
  } else if (customers.data) {
    body = (
      <QueueTable
        rows={customers.data.items}
        asOf={asOf}
        sort={params.sort}
        order={params.order}
        onSort={onSort}
        onOpen={open}
      />
    );
  }

  return (
    <Stack gap="lg">
      <Group justify="space-between" align="center">
        <Title order={1}>Очередь удержания</Title>
        <Button
          variant={whatIf ? 'filled' : 'default'}
          aria-expanded={whatIf}
          onClick={() => setWhatIf((v) => !v)}
        >
          Что если
        </Button>
      </Group>
      {whatIf ? <WhatIfPanel role={role} /> : null}
      {customers.isError && customers.data ? (
        <StateOffline onRetry={() => void customers.refetch()} />
      ) : null}
      <SummaryCards
        expectedLossCents={summary.data?.expectedLossCents ?? null}
        atRiskCents={summary.data?.atRiskCents ?? null}
        lostCents={summary.data?.lostCents ?? null}
      />
      {unfiltered.data && asOf ? (
        <DeletionStrip rows={unfiltered.data.items} asOf={asOf} onOpen={open} />
      ) : null}
      <Group gap="md" align="flex-end">
        <TextInput
          label="Поиск по email"
          placeholder="user@example.test"
          leftSection={<IconSearch size={16} aria-hidden />}
          value={search}
          onChange={(e) => setSearch(e.currentTarget.value)}
        />
        <Select
          label="Уровень риска"
          placeholder="Все"
          clearable
          data={levelOptions}
          value={params.level ?? null}
          onChange={(value) =>
            onParams({ ...params, level: (value as RiskLevel | null) ?? undefined })
          }
        />
      </Group>
      {body}
    </Stack>
  );
}
