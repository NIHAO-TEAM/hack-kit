import { Button, Group, Select, Stack, Text, TextInput, Title } from '@mantine/core';
import { useDebouncedValue } from '@mantine/hooks';
import { keepPreviousData, useQuery } from '@tanstack/react-query';
import { useRouter } from '@tanstack/react-router';
import { useEffect, useState } from 'react';
import {
  customersExportHref,
  customersOptions,
  QUEUE_PAGE_SIZE,
  type CustomerParams,
  type CustomerSort,
  type RiskLevel,
} from '@entities/customer';
import { ApiError } from '@shared/api';
import type { Role } from '@entities/session';
import {
  IconDownload,
  IconSearch,
  Pagination,
  StateEmpty,
  StateError,
  StateForbidden,
  StateLoading,
  StateOffline,
} from '@shared/ui';
import { featureTitle } from '@entities/model-config';
import { summaryOptions } from '@entities/summary';
import { BillingGroup } from './BillingGroup';
import { DeletionStrip } from './DeletionStrip';
import { QueueTable } from './QueueTable';
import { SortControl } from './SortControl';
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
  reasonLoss: 'desc',
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
  const page = params.page ?? 1;
  const customers = useQuery({
    ...customersOptions({ ...params, billing: 'exclude', limit: QUEUE_PAGE_SIZE }),
    placeholderData: keepPreviousData,
  });
  const total = customers.data?.total ?? 0;
  const pageCount = Math.max(Math.ceil(total / QUEUE_PAGE_SIZE), 1);
  // Смена вида возвращает на первую страницу.
  const withView = (next: CustomerParams): CustomerParams => ({ ...next, page: undefined });
  const toPage = (value: number) => onParams({ ...params, page: value > 1 ? value : undefined });
  // Страница за пределами total (ссылка устарела) — первая.
  useEffect(() => {
    if (customers.data && !customers.isPlaceholderData && page > pageCount) toPage(1);
    // eslint-disable-next-line react-hooks/exhaustive-deps -- реагируем на ответ сервера
  }, [customers.data, customers.isPlaceholderData]);
  const summary = useQuery(summaryOptions);
  // Полоса «удалит биллинг» не зависит от поиска и фильтра: к биллингу уходят все.
  const unfiltered = useQuery(
    customersOptions({ sort: 'expectedLoss', order: 'desc', billing: 'exclude' }),
  );
  // Группа «У биллинга»: сбой автосписания, маркетолог не действует; сортировка и поиск очереди не влияют.
  const billing = useQuery(
    customersOptions({ sort: 'expectedLoss', order: 'desc', billing: 'only' }),
  );

  // Причины — те, что есть в сводке «Где теряем деньги»; выбранная из адреса остаётся в списке всегда.
  const reasonIds = (summary.data?.lossByFeature ?? []).map((f) => f.featureId);
  if (params.reason && !reasonIds.includes(params.reason)) reasonIds.push(params.reason);
  const reasonOptions = reasonIds.map((id) => ({ value: id, label: featureTitle(id) }));

  const [whatIf, setWhatIf] = useState(false);
  const [search, setSearch] = useState(params.q ?? '');
  const [debounced] = useDebouncedValue(search, 300);
  useEffect(() => {
    if (debounced !== (params.q ?? ''))
      onParams(withView({ ...params, q: debounced || undefined }));
    // eslint-disable-next-line react-hooks/exhaustive-deps -- реагируем только на ввод
  }, [debounced]);

  const open = (id: number) => void router.navigate({ href: `/queue/${id}` });
  const onSort = (column: CustomerSort) => {
    // Колонка «Потеря» при сортировке по потере по причине считается активной: клик меняет направление.
    const target =
      column === 'expectedLoss' && params.sort === 'reasonLoss' ? 'reasonLoss' : column;
    onParams(
      withView({
        ...params,
        sort: target,
        order:
          params.sort === target ? (params.order === 'asc' ? 'desc' : 'asc') : firstOrder[target],
      }),
    );
  };

  const onToggleOrder = () =>
    onParams(withView({ ...params, order: params.order === 'asc' ? 'desc' : 'asc' }));

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
      params.q || params.level || params.reason ? (
        <StateEmpty title="Никого не нашли">Измените поиск, уровень риска или причину.</StateEmpty>
      ) : (
        <StateEmpty title="Сегодня некого спасать">
          Клиентов под риском нет. Очередь обновится сама, когда данные изменятся.
        </StateEmpty>
      );
  } else if (customers.data) {
    body = (
      <Stack gap="sm" style={{ opacity: customers.isPlaceholderData ? 0.6 : 1 }}>
        <SortControl
          sort={params.sort}
          order={params.order}
          onField={(column) =>
            onParams(withView({ ...params, sort: column, order: firstOrder[column] }))
          }
          onToggleOrder={onToggleOrder}
          reason={params.reason}
        />
        <QueueTable
          rows={customers.data.items}
          version={customers.data.version}
          asOf={asOf}
          sort={params.sort}
          order={params.order}
          onSort={onSort}
          onOpen={open}
        />
        {total > QUEUE_PAGE_SIZE ? (
          <Group justify="space-between" align="center">
            <Text size="sm" c="dimmed" aria-live="polite">
              Показаны {(page - 1) * QUEUE_PAGE_SIZE + 1}–
              {(page - 1) * QUEUE_PAGE_SIZE + customers.data.items.length} из {total}
            </Text>
            <Pagination value={page} onChange={toPage} total={pageCount} />
          </Group>
        ) : null}
      </Stack>
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
            onParams(withView({ ...params, level: (value as RiskLevel | null) ?? undefined }))
          }
        />
        <Select
          label="Причина"
          placeholder="Все"
          clearable
          data={reasonOptions}
          value={params.reason ?? null}
          onChange={(value) =>
            onParams(
              withView({
                ...params,
                reason: value ?? undefined,
                // Смена причины возвращает порядок по умолчанию: по потере по ней, а без неё — по потере клиента.
                sort: value
                  ? 'reasonLoss'
                  : params.sort === 'reasonLoss'
                    ? 'expectedLoss'
                    : params.sort,
                order: params.sort === 'reasonLoss' || !value ? 'desc' : params.order,
              }),
            )
          }
        />
        <Button
          component="a"
          href={customersExportHref(params)}
          download
          variant="default"
          leftSection={<IconDownload size={16} aria-hidden />}
        >
          Скачать CSV
        </Button>
      </Group>
      {body}
      <BillingGroup
        rows={billing.data?.items}
        total={billing.data?.total}
        loading={billing.isPending}
        failed={billing.isError && !billing.data}
        onRetry={() => void billing.refetch()}
        onOpen={open}
      />
    </Stack>
  );
}
