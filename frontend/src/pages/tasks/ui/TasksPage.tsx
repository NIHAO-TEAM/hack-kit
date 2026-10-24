import { Group, SegmentedControl, Stack, Table, Text, Title, Tooltip } from '@mantine/core';
import { useQuery } from '@tanstack/react-query';
import { useEffect, useState } from 'react';
import { customerQuery } from '@entities/customer-detail';
import { customersOptions, type CustomerRow } from '@entities/customer';
import {
  CustomerLink,
  dueText,
  statusLabel,
  tasksQuery,
  TASKS_LIMIT,
  type RetentionTask,
  type TaskFilter,
} from '@entities/retention-task';
import type { Role } from '@entities/session';
import { ApiError } from '@shared/api';
import { formatMoney } from '@shared/lib';
import { StateEmpty, StateError, StateForbidden, StateLoading, StateOffline } from '@shared/ui';

const filters: Array<{ value: TaskFilter; label: string }> = [
  { value: 'mine', label: 'Мои' },
  { value: 'open', label: 'Все открытые' },
  { value: 'overdue', label: 'Просроченные' },
  { value: 'closed', label: 'Закрытые' },
];

/** Срок наступает и без событий: подпись пересчитывается раз в минуту. */
function useNow(): number {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const timer = setInterval(() => setNow(Date.now()), 60_000);
    return () => clearInterval(timer);
  }, []);
  return now;
}

const exact = (iso: string) =>
  new Date(iso).toLocaleString('ru-RU', {
    timeZone: 'Europe/Moscow',
    dateStyle: 'medium',
    timeStyle: 'short',
  }) + ' МСК';

function Client({ task }: { task: RetentionTask }) {
  const label = `${task.customerId}${task.customerEmail ? ` · ${task.customerEmail}` : ''}`;
  if (task.customerMissing) {
    return (
      <Stack gap={0}>
        <Text size="sm">{label}</Text>
        <Text size="xs" c="dimmed">
          клиента нет в текущем снимке
        </Text>
      </Stack>
    );
  }
  return <CustomerLink id={task.customerId}>{label}</CustomerLink>;
}

function Due({ task, now }: { task: RetentionTask; now: number }) {
  if (task.status === 'closed') {
    return <Text size="sm">{task.closedAt ? exact(task.closedAt) : '—'}</Text>;
  }
  const due = dueText(task.dueAt, now);
  return (
    <Tooltip label={exact(task.dueAt)}>
      <Text
        size="sm"
        tabIndex={0}
        c={due.overdue ? 'var(--lost-fg)' : undefined}
        fw={due.overdue ? 600 : 400}
      >
        {due.overdue ? '⚠ ' : ''}
        {due.text}
      </Text>
    </Tooltip>
  );
}

function Risk({ task, listed }: { task: RetentionTask; listed?: CustomerRow }) {
  // Список клиентов — одна страница; клиента за её пределами берём из карточки, а не показываем «—».
  const detail = useQuery({
    ...customerQuery(task.customerId),
    enabled: !listed && !task.customerMissing,
  });
  const current = listed ?? detail.data?.customer;
  const then = task.baseline.score != null ? Math.round(task.baseline.score) : null;
  const now = current?.score != null ? Math.round(current.score) : null;
  return (
    <Stack gap={0}>
      <Text size="sm">
        сейчас: {now ?? '—'}
        {current ? `, ${formatMoney(current.expectedLossCents)}` : ''}
      </Text>
      <Text size="xs" c="dimmed">
        при взятии: {then ?? '—'}, {formatMoney(task.baseline.expectedLossCents)}
      </Text>
    </Stack>
  );
}

export function TasksPage({ role, initialFilter }: { role: Role; initialFilter?: TaskFilter }) {
  const [filter, setFilter] = useState<TaskFilter>(
    initialFilter ?? (role === 'marketer' ? 'mine' : 'open'),
  );
  const tasks = useQuery(tasksQuery(filter));
  const customers = useQuery(customersOptions({ sort: 'id', order: 'asc' }));
  const now = useNow();
  const byId = new Map<number, CustomerRow>((customers.data?.items ?? []).map((c) => [c.id, c]));
  const error = tasks.error;

  let body;
  if (error instanceof ApiError && error.status === 403) {
    body = <StateForbidden>Задачи доступны не всем ролям.</StateForbidden>;
  } else if (tasks.isPending) {
    body = <StateLoading label="Загрузка задач" />;
  } else if (tasks.isError) {
    body =
      error instanceof ApiError && error.isNetwork ? (
        <StateOffline onRetry={() => void tasks.refetch()} />
      ) : (
        <StateError
          title="Не удалось загрузить задачи"
          requestId={error instanceof ApiError ? error.requestId : null}
          onRetry={() => void tasks.refetch()}
        >
          Сервер ответил ошибкой. Повторите попытку.
        </StateError>
      );
  } else if (tasks.data.items.length === 0) {
    body = (
      <StateEmpty title="Задач нет">
        Задач нет. Возьмите клиента в работу из очереди или карточки.
      </StateEmpty>
    );
  } else {
    body = (
      <Stack gap="xs">
        {tasks.data.total > TASKS_LIMIT ? (
          <Text size="sm" c="dimmed">
            Показаны первые {TASKS_LIMIT} из {tasks.data.total}, новые правки сверху.
          </Text>
        ) : null}
        <Table.ScrollContainer minWidth={760}>
          <Table aria-label="Задачи удержания" verticalSpacing="sm">
            <Table.Thead>
              <Table.Tr>
                <Table.Th>Клиент</Table.Th>
                <Table.Th>Статус</Table.Th>
                <Table.Th>Ответственный</Table.Th>
                <Table.Th>{filter === 'closed' ? 'Закрыта' : 'Срок'}</Table.Th>
                <Table.Th>Скор и потеря</Table.Th>
                <Table.Th>Последнее событие</Table.Th>
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {tasks.data.items.map((t) => (
                <Table.Tr key={t.id}>
                  <Table.Td>
                    <Client task={t} />
                  </Table.Td>
                  <Table.Td>{statusLabel[t.status]}</Table.Td>
                  <Table.Td>{t.assignee}</Table.Td>
                  <Table.Td>
                    <Due task={t} now={now} />
                  </Table.Td>
                  <Table.Td>
                    <Risk task={t} listed={byId.get(t.customerId)} />
                  </Table.Td>
                  <Table.Td>
                    <Text size="sm">{exact(t.updatedAt)}</Text>
                    <Text size="xs" c="dimmed">
                      {t.updatedBy}
                    </Text>
                  </Table.Td>
                </Table.Tr>
              ))}
            </Table.Tbody>
          </Table>
        </Table.ScrollContainer>
      </Stack>
    );
  }

  return (
    <Stack gap="lg">
      <Group justify="space-between" align="center">
        <Title order={1}>Задачи</Title>
      </Group>
      <SegmentedControl
        aria-label="Фильтр задач"
        data={filters}
        value={filter}
        onChange={(v) => setFilter(v as TaskFilter)}
      />
      {body}
    </Stack>
  );
}
