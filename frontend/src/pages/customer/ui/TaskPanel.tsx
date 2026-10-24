import {
  Alert,
  Badge,
  Button,
  Card,
  Group,
  List,
  Select,
  Stack,
  Text,
  Textarea,
  TextInput,
  Title,
  Tooltip,
} from '@mantine/core';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useRef, useState } from 'react';
import {
  TaskConflictError,
  addTaskEvent,
  assigneesQuery,
  channelLabel,
  dueText,
  fromLocalInput,
  OBSERVATION_NOTE,
  observations,
  outcomeLabel,
  patchTask,
  statusLabel,
  suggestedOutcome,
  takeTask,
  taskKey,
  taskQuery,
  toLocalInput,
  type RetentionTask,
  type TaskBrief,
  type TaskChannel,
  type TaskEvent,
  type TaskOutcome,
} from '@entities/retention-task';
import { StateError, StateLoading } from '@shared/ui';

const when = new Intl.DateTimeFormat('ru-RU', { dateStyle: 'short', timeStyle: 'short' });
const whenLong = new Intl.DateTimeFormat('ru-RU', { dateStyle: 'long', timeStyle: 'short' });

const channelOptions = Object.entries(channelLabel).map(([value, label]) => ({ value, label }));
const outcomeOptions = Object.entries(outcomeLabel).map(([value, label]) => ({ value, label }));

type Mode = 'contact' | 'postpone' | 'close' | null;

function eventText(e: TaskEvent): string {
  switch (e.kind) {
    case 'created':
      return 'взята в работу';
    case 'contact':
      return `связались${e.channel ? ` (${channelLabel[e.channel].toLowerCase()})` : ''}${e.text ? `: ${e.text}` : ''}`;
    case 'note':
      return `заметка: ${e.text}`;
    case 'observed':
      return e.text;
    default:
      return e.text || 'изменена задача';
  }
}

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

function History({ events }: { events: readonly TaskEvent[] }) {
  // Наблюдения биллинга живут в своём разделе, в истории действий сотрудников их нет.
  events = events.filter((e) => e.kind !== 'observed');
  if (events.length === 0) return null;
  return (
    <Stack gap={4}>
      <Title order={3} size="h5">
        История
      </Title>
      <List listStyleType="none" spacing={4} aria-label="История задачи">
        {[...events].reverse().map((e) => (
          <List.Item key={e.id}>
            <Text size="sm">
              {when.format(new Date(e.at))} · {e.actor} · {eventText(e)}
            </Text>
          </List.Item>
        ))}
      </List>
    </Stack>
  );
}

/** Срок словами; просроченный — цветом внимания, точная дата — в подсказке. */
function Due({ dueAt, closed }: { dueAt: string; closed: boolean }) {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const timer = setInterval(() => setNow(Date.now()), 60_000);
    return () => clearInterval(timer);
  }, []);
  const { text, overdue } = dueText(dueAt, now);
  const warn = overdue && !closed;
  return (
    <Tooltip label={whenLong.format(new Date(dueAt))}>
      <Text span size="sm" c={warn ? 'orange' : 'dimmed'} fw={warn ? 600 : undefined} tabIndex={0}>
        {text}
      </Text>
    </Tooltip>
  );
}

/**
 * «Работа с клиентом» в карточке: взять в работу, контакт, перенос срока, закрытие, история.
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
  const assignees = useQuery({ ...assigneesQuery, enabled: canEdit });
  const taskQ = useQuery({ ...taskQuery(brief?.id ?? 0), enabled: brief !== undefined });
  const [mode, setMode] = useState<Mode>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [conflict, setConflict] = useState<TaskConflictError | null>(null);
  // Введённое хранится отдельно от сервера: после 409 оно остаётся на месте.
  const [assigneeDraft, setAssigneeDraft] = useState<string | null>(null);
  const [dueDraft, setDueDraft] = useState<string | null>(null);
  const [noteDraft, setNoteDraft] = useState<string | null>(null);
  const [channel, setChannel] = useState<TaskChannel | null>(null);
  const [contactText, setContactText] = useState('');
  const [postponeTo, setPostponeTo] = useState('');
  // Контакт уже записан, а смена статуса не удалась: повтор не пишет событие второй раз.
  const recorded = useRef(false);
  const [outcome, setOutcome] = useState<TaskOutcome | null>(null);

  const assigneeOptions = (assignees.data ?? []).map((a) => ({ value: a.login, label: a.login }));

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
        <Alert color="orange" role="alert" title="Задача изменилась">
          <Group justify="space-between" wrap="wrap">
            <Text size="sm">
              Задачу уже изменил {conflict.changedBy ?? 'другой сотрудник'}. Введённое сохранено.
            </Text>
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
              <Group align="flex-end" wrap="wrap">
                <Select
                  label="Ответственный"
                  placeholder="Я"
                  data={assigneeOptions}
                  value={assigneeDraft}
                  onChange={setAssigneeDraft}
                  clearable
                />
                <TextInput
                  label="Срок"
                  type="datetime-local"
                  description="Пусто — завтра 18:00"
                  value={dueDraft ?? ''}
                  onChange={(e) => setDueDraft(e.currentTarget.value)}
                />
                <Button
                  size="lg"
                  loading={busy}
                  onClick={() =>
                    void run(() =>
                      takeTask(customerId, {
                        assignee: assigneeDraft ?? undefined,
                        dueAt: fromLocalInput(dueDraft ?? ''),
                      }),
                    )
                  }
                >
                  Взять в работу
                </Button>
              </Group>
              {alerts}
            </>
          ) : (
            <Text c="dimmed">Задачи по клиенту нет.</Text>
          )}
        </Stack>
      </Card>
    );
  }

  if (taskQ.isPending) return <StateLoading label="Загрузка задачи" />;
  const task = taskQ.data;
  if (!task) {
    return (
      <StateError title="Не удалось загрузить задачу" onRetry={() => void taskQ.refetch()}>
        {taskQ.error?.message}
      </StateError>
    );
  }

  const assigneeValue = assigneeDraft ?? task.assignee;
  const dueValue = dueDraft ?? toLocalInput(task.dueAt);
  const noteValue = noteDraft ?? task.note;
  const dirty =
    assigneeValue !== task.assignee ||
    noteValue !== task.note ||
    new Date(fromLocalInput(dueValue) ?? task.dueAt).getTime() !== new Date(task.dueAt).getTime();
  const resetDrafts = () => {
    setAssigneeDraft(null);
    setDueDraft(null);
    setNoteDraft(null);
  };
  const closeMode = () => setMode(null);

  const save = () =>
    run(
      () =>
        patchTask(task.id, {
          version: task.version,
          assignee: assigneeValue !== task.assignee ? assigneeValue : undefined,
          dueAt: fromLocalInput(dueValue),
          note: noteValue !== task.note ? noteValue : undefined,
        }),
      resetDrafts,
    );

  const contact = () =>
    run(
      async () => {
        // Версия для PATCH — у актуальной задачи: после «Показать новое» она свежая.
        let next = task;
        if (!recorded.current) {
          next = await addTaskEvent(task.id, {
            kind: 'contact',
            channel: channel ?? undefined,
            text: contactText,
          });
          recorded.current = true;
        }
        if (next.status !== 'contacted') {
          next = await patchTask(task.id, { version: next.version, status: 'contacted' });
        }
        recorded.current = false;
        return next;
      },
      () => {
        closeMode();
        setContactText('');
        setChannel(null);
      },
    );

  const postpone = () =>
    run(
      () =>
        patchTask(task.id, {
          version: task.version,
          status: 'postponed',
          dueAt: fromLocalInput(postponeTo),
        }),
      () => {
        closeMode();
        setPostponeTo('');
        resetDrafts();
      },
    );

  const close = () =>
    run(
      () =>
        patchTask(task.id, {
          version: task.version,
          status: 'closed',
          outcome: outcome ?? undefined,
        }),
      closeMode,
    );

  return (
    <Card withBorder radius="sm" padding="md" component="section" aria-label="Работа с клиентом">
      <Stack gap="sm">
        {title}
        <Group gap="sm" align="center" wrap="wrap">
          <Badge variant="light">{statusLabel[task.status]}</Badge>
          <Text size="sm">Ответственный: {task.assignee}</Text>
          <Due dueAt={task.dueAt} closed={task.status === 'closed'} />
        </Group>
        {canEdit ? (
          <>
            <Group align="flex-end" wrap="wrap">
              <Select
                label="Ответственный"
                data={
                  assigneeOptions.some((o) => o.value === assigneeValue)
                    ? assigneeOptions
                    : [...assigneeOptions, { value: assigneeValue, label: assigneeValue }]
                }
                value={assigneeValue}
                onChange={(v) => setAssigneeDraft(v)}
                allowDeselect={false}
              />
              <TextInput
                label="Срок"
                type="datetime-local"
                value={dueValue}
                onChange={(e) => setDueDraft(e.currentTarget.value)}
              />
            </Group>
            <Textarea
              label="Заметка"
              rows={2}
              maxLength={2000}
              value={noteValue}
              onChange={(e) => setNoteDraft(e.currentTarget.value)}
            />
            <Group wrap="wrap">
              <Button
                size="lg"
                variant="default"
                disabled={!dirty}
                loading={busy}
                onClick={() => void save()}
              >
                Сохранить
              </Button>
              <Button size="lg" variant="default" onClick={() => setMode('contact')}>
                Связались
              </Button>
              <Button size="lg" variant="default" onClick={() => setMode('postpone')}>
                Отложить до…
              </Button>
              <Button
                size="lg"
                variant="default"
                onClick={() => {
                  // Продление или автопродление в наблюдениях — итог «Продлил» по умолчанию, выбирает человек.
                  setOutcome((cur) => cur ?? suggestedOutcome(task.events ?? []));
                  setMode('close');
                }}
              >
                Закрыть
              </Button>
            </Group>
            {mode === 'contact' ? (
              <Group align="flex-end" wrap="wrap">
                <Select
                  label="Канал"
                  data={channelOptions}
                  value={channel}
                  onChange={(v) => setChannel(v as TaskChannel | null)}
                />
                <Textarea
                  label="Что сказали"
                  rows={1}
                  maxLength={2000}
                  value={contactText}
                  onChange={(e) => setContactText(e.currentTarget.value)}
                />
                <Button size="lg" disabled={!channel} loading={busy} onClick={() => void contact()}>
                  Записать контакт
                </Button>
              </Group>
            ) : null}
            {mode === 'postpone' ? (
              <Group align="flex-end" wrap="wrap">
                <TextInput
                  label="Новый срок"
                  type="datetime-local"
                  value={postponeTo}
                  onChange={(e) => setPostponeTo(e.currentTarget.value)}
                />
                <Button
                  size="lg"
                  disabled={!fromLocalInput(postponeTo)}
                  loading={busy}
                  onClick={() => void postpone()}
                >
                  Отложить
                </Button>
              </Group>
            ) : null}
            {mode === 'close' ? (
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
            ) : null}
            {alerts}
          </>
        ) : task.note ? (
          <Text size="sm">Заметка: {task.note}</Text>
        ) : null}
        <Observed events={task.events ?? []} />
        <History events={task.events ?? []} />
      </Stack>
    </Card>
  );
}
