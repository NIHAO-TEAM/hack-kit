import { Alert, Badge, Button, Card, Group, List, Select, Stack, Text, Title } from '@mantine/core';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import {
  TaskConflictError,
  OBSERVATION_NOTE,
  observations,
  outcomeLabel,
  patchTask,
  statusLabel,
  suggestedOutcome,
  takeTask,
  taskKey,
  taskQuery,
  type RetentionTask,
  type TaskBrief,
  type TaskEvent,
  type TaskOutcome,
} from '@entities/retention-task';
import { StateError, StateLoading } from '@shared/ui';

const when = new Intl.DateTimeFormat('ru-RU', { dateStyle: 'short', timeStyle: 'short' });

/** Итогов три (ADR-0004): «Не нужно» остаётся в данных, но в интерфейсе не предлагается. */
const outcomeOptions = (['renewed', 'refused', 'no_answer'] as const).map((value) => ({
  value,
  label: outcomeLabel[value],
}));

/** «После взятия в работу»: что показал биллинг. Подпись обязательна — это «после», а не «благодаря». */
function Observed({ events }: { events: readonly TaskEvent[] }) {
  const list = observations(events);
  if (list.length === 0) return null;
  return (
    <Stack gap={4} component="section" aria-label="После взятия в работу">
      <Title order={3} size="h5">
        После взятия в работу
      </Title>
      <List listStyleType="none" spacing={4} aria-label="Наблюдения биллинга">
        {list.map((e) => (
          <List.Item key={e.id}>
            <Text size="sm">
              {when.format(new Date(e.at))} · {e.text}
            </Text>
          </List.Item>
        ))}
      </List>
      <Text size="xs" c="dimmed">
        {OBSERVATION_NOTE}
      </Text>
    </Stack>
  );
}

/**
 * Одна цепочка в карточке (ADR-0004): «Отметить: взято в работу» → «Закрыть» с итогом →
 * наблюдения биллинга. Срок, ответственный, контакты и заметки в интерфейсе не показываются.
 * Без права `tasks.edit` блок только показывает состояние.
 */
export function TaskPanel({
  customerId,
  brief,
  canEdit,
}: {
  customerId: number;
  brief: TaskBrief | undefined;
  canEdit: boolean;
}) {
  const queryClient = useQueryClient();
  const taskQ = useQuery({ ...taskQuery(brief?.id ?? 0), enabled: brief !== undefined });
  const [closing, setClosing] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [conflict, setConflict] = useState<TaskConflictError | null>(null);
  const [outcome, setOutcome] = useState<TaskOutcome | null>(null);

  const refreshCard = () => {
    void queryClient.invalidateQueries({ queryKey: ['customer', customerId] });
    void queryClient.invalidateQueries({ queryKey: ['customers'] });
  };

  async function run(action: () => Promise<RetentionTask>, done?: () => void) {
    setBusy(true);
    setError(null);
    setConflict(null);
    try {
      const task = await action();
      queryClient.setQueryData(taskKey(task.id), task);
      refreshCard();
      done?.();
    } catch (e) {
      if (e instanceof TaskConflictError) setConflict(e);
      else setError(e instanceof Error ? e.message : 'Не удалось сохранить');
    } finally {
      setBusy(false);
    }
  }

  const showNew = () => {
    if (conflict?.task) queryClient.setQueryData(taskKey(conflict.task.id), conflict.task);
    refreshCard();
    setConflict(null);
  };

  const alerts = (
    <>
      {conflict ? (
        <Alert color="orange" role="alert" title="Отметка изменилась">
          <Group justify="space-between" wrap="wrap">
            <Text size="sm">Отметку уже изменил {conflict.changedBy ?? 'другой сотрудник'}.</Text>
            <Button variant="default" size="lg" onClick={showNew}>
              Показать новое
            </Button>
          </Group>
        </Alert>
      ) : null}
      {error ? (
        <Text c="red" role="alert" size="sm">
          {error}
        </Text>
      ) : null}
    </>
  );

  const title = (
    <Title order={2} size="h4">
      Работа с клиентом
    </Title>
  );

  if (brief === undefined) {
    return (
      <Card withBorder radius="sm" padding="md" component="section" aria-label="Работа с клиентом">
        <Stack gap="sm">
          {title}
          {canEdit ? (
            <>
              <Group>
                <Button
                  size="lg"
                  loading={busy}
                  // Срок и ответственного назначает сервер по умолчанию: в интерфейсе их нет.
                  onClick={() => void run(() => takeTask(customerId, {}))}
                >
                  Отметить: взято в работу
                </Button>
              </Group>
              {alerts}
            </>
          ) : (
            <Text c="dimmed">Клиента в работу не брали.</Text>
          )}
        </Stack>
      </Card>
    );
  }

  if (taskQ.isPending) return <StateLoading label="Загрузка отметки" />;
  const task = taskQ.data;
  if (!task) {
    return (
      <StateError title="Не удалось загрузить отметку" onRetry={() => void taskQ.refetch()}>
        {taskQ.error?.message}
      </StateError>
    );
  }

  const close = () =>
    run(
      () =>
        patchTask(task.id, {
          version: task.version,
          status: 'closed',
          outcome: outcome ?? undefined,
        }),
      () => setClosing(false),
    );

  return (
    <Card withBorder radius="sm" padding="md" component="section" aria-label="Работа с клиентом">
      <Stack gap="sm">
        {title}
        <Group gap="sm" align="center" wrap="wrap">
          <Badge variant="light">
            {task.status === 'closed' ? statusLabel.closed : 'Взято в работу'}
          </Badge>
          {task.status === 'closed' && task.outcome ? (
            <Text size="sm">Итог: {outcomeLabel[task.outcome]}</Text>
          ) : null}
        </Group>
        {canEdit && task.status !== 'closed' ? (
          <>
            {!closing ? (
              <Group wrap="wrap">
                <Button
                  size="lg"
                  variant="default"
                  onClick={() => {
                    // Продление или автопродление в наблюдениях — итог «Продлил» по умолчанию, выбирает человек.
                    setOutcome((cur) => cur ?? suggestedOutcome(task.events ?? []));
                    setClosing(true);
                  }}
                >
                  Закрыть
                </Button>
              </Group>
            ) : (
              <Group align="flex-end" wrap="wrap">
                <Select
                  label="Итог"
                  data={outcomeOptions}
                  value={outcome}
                  onChange={(v) => setOutcome(v as TaskOutcome | null)}
                />
                <Button size="lg" disabled={!outcome} loading={busy} onClick={() => void close()}>
                  Закрыть задачу
                </Button>
              </Group>
            )}
            {alerts}
          </>
        ) : null}
        <Observed events={task.events ?? []} />
      </Stack>
    </Card>
  );
}
