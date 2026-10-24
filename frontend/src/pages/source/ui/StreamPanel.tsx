import {
  Alert,
  Badge,
  Button,
  Group,
  Paper,
  Progress,
  Select,
  Stack,
  Table,
  Text,
  Title,
} from '@mantine/core';
import { useMutation, useQuery } from '@tanstack/react-query';
import { useDeferredValue, useState } from 'react';
import {
  formatCount,
  formatDuration,
  formatMs,
  formatRate,
  progressPercent,
  startStream,
  streamQuery,
  type StreamReport,
} from '@entities/stream-run';
import { ApiError } from '@shared/api';
import { formatMoney } from '@shared/lib';
import { LevelBadge } from '@shared/ui';
import { emulatorActions } from '../api/source';
import { totalsLine } from '../model/window';
import { LiveCharts } from './LiveCharts';
import { useAfterWrite } from './useAfterWrite';

const DEFAULT_EVENTS = '2500';
const VOLLEYS = ['2500', '10000', '50000', '100000'].map((v) => ({
  value: v,
  label: formatCount(Number(v)),
}));
const DEFAULT_RATE = '0';
const RATES = [
  { value: '0', label: 'Как можно быстрее' },
  { value: '50', label: '50 в секунду' },
  { value: '100', label: '100 в секунду' },
  { value: '500', label: '500 в секунду' },
  { value: '2000', label: '2 000 в секунду' },
];

function Row({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <Table.Tr>
      <Table.Th scope="row" fw={400}>
        {label}
      </Table.Th>
      <Table.Td ta="right">{children}</Table.Td>
    </Table.Tr>
  );
}

/** Итоговая таблица прогона и отметка согласованности. */
function Report({ report }: { report: StreamReport }) {
  const c = report.consistency;
  return (
    <Stack gap="sm" aria-label="Итог прогона" component="section">
      {c.ok ? (
        <Alert color="teal" role="status">
          Согласованность: пересчёт с нуля совпал (разница {c.diffCents} ¢, снимок {c.version})
        </Alert>
      ) : (
        <Alert color="red" role="alert">
          Согласованность: расхождение {formatMoney(c.diffCents)} между сервисом и пересчётом с нуля
          (снимок {c.version})
        </Alert>
      )}
      <Text size="sm" fw={500} role="status">
        {totalsLine(report)}
      </Text>
      <Table withRowBorders aria-label="Результаты потока событий">
        <Table.Tbody>
          <Row label="Применено событий">
            {formatCount(report.applied)} из {formatCount(report.events)}
            {report.failed > 0 ? `, не применено ${formatCount(report.failed)}` : ''}
          </Row>
          <Row label="Длительность">{formatDuration(report.durationMs)}</Row>
          <Row label="Скорость">{formatRate(report.eventsPerSecond)}</Row>
          <Row label="Применение события, p50 / p95 / максимум">
            {formatMs(report.applyP50Ms)} / {formatMs(report.applyP95Ms)} /{' '}
            {formatMs(report.applyMaxMs)}
          </Row>
          <Row label="Пересчётов выполнено">{formatCount(report.recalcs)}</Row>
          <Row label="Событие → обновлённая очередь, p95">
            {report.queueLagP95Ms === undefined ? '—' : formatMs(report.queueLagP95Ms)}
          </Row>
          <Row label="Ожидаемая потеря до → после">
            {formatMoney(report.lossBeforeCents)} → {formatMoney(report.lossAfterCents)}
          </Row>
          <Row label="Клиенты по уровням до → после">
            {(['high', 'medium', 'low', 'unscored', 'lost'] as const)
              .map((l) => `${report.levelsBefore[l]} → ${report.levelsAfter[l]}`)
              .join(' · ')}
          </Row>
        </Table.Tbody>
      </Table>
      <Text size="sm" c="dimmed">
        Порядок уровней: высокий · средний · низкий · без оценки · потерян.
      </Text>
      {report.levelChanges.length > 0 ? (
        <Stack gap={4}>
          <Text size="sm" fw={500}>
            Сменили уровень
          </Text>
          {report.levelChanges.map((ch) => (
            <Group key={`${ch.from}-${ch.to}`} gap="xs">
              <Text size="sm">{formatCount(ch.count)} кл.:</Text>
              <LevelBadge level={ch.from} />
              <Text size="sm" aria-hidden>
                →
              </Text>
              <LevelBadge level={ch.to} />
            </Group>
          ))}
        </Stack>
      ) : (
        <Text size="sm" c="dimmed">
          Уровни клиентов не изменились.
        </Text>
      )}
    </Stack>
  );
}

/**
 * Поток событий пользователей: сервер подаёт N событий в правки услуг и счетов, очередь
 * маркетолога обновляется по ходу, в конце — отчёт и сверка с пересчётом с нуля.
 */
export function StreamPanel() {
  const after = useAfterWrite();
  const stream = useQuery(streamQuery);
  const [events, setEvents] = useState(DEFAULT_EVENTS);
  const [rate, setRate] = useState(DEFAULT_RATE);

  const count = Number(events);
  const state = stream.data;
  const liveWindow = useDeferredValue(state?.window);
  const running = state?.state === 'running';

  const start = useMutation({
    mutationFn: () => startStream({ events: count, ratePerSecond: Number(rate), seed: 1 }),
    onSuccess: () => void stream.refetch(),
    onError: (e) => after.fail('Поток не запущен', e),
  });
  const reset = useMutation({
    mutationFn: () => emulatorActions([{ actionId: 'reset', customerId: 0 }]),
    onSuccess: () => after.done('Данные возвращены к набору организаторов'),
    onError: (e) => after.fail('Сброс не выполнен', e),
  });

  return (
    <Paper withBorder p="lg" radius="lg" component="section" aria-label="Поток событий">
      <Stack gap="sm">
        <Title order={2} size="h4">
          Поток событий
        </Title>
        <Text size="sm" c="dimmed">
          Имитация потока: пополнения, автопродление, остановка услуг и неудачные продления идут в
          те же правки, что и вручную. Данные стенда меняются — после показа нажмите «Сбросить».
        </Text>
        <Group align="flex-end" wrap="wrap">
          <Select
            label="Залп, событий"
            data={VOLLEYS}
            value={events}
            onChange={(v) => setEvents(v ?? DEFAULT_EVENTS)}
            allowDeselect={false}
            disabled={running}
            w={170}
          />
          <Select
            label="Скорость"
            data={RATES}
            value={rate}
            onChange={(v) => setRate(v ?? DEFAULT_RATE)}
            allowDeselect={false}
            disabled={running}
            w={200}
          />
        </Group>
        <Button loading={start.isPending || running} onClick={() => start.mutate()}>
          Прогнать {formatCount(count)} событий
        </Button>

        {stream.isError ? (
          <Alert color="red" role="alert">
            {stream.error instanceof ApiError
              ? stream.error.message
              : 'Состояние потока не получено'}
          </Alert>
        ) : null}

        {running && state ? (
          <Stack gap={4}>
            <Progress value={progressPercent(state)} aria-label="Ход потока событий" animated />
            <Text size="sm" role="status">
              Применено {formatCount(state.applied)} из {formatCount(state.events)}
            </Text>
          </Stack>
        ) : null}

        {liveWindow && (running || state?.state === 'done') ? (
          <LiveCharts window={liveWindow} />
        ) : null}

        {state?.state === 'failed' ? (
          <Alert color="red" role="alert" title="Прогон не завершён">
            {state.error ?? 'Причина не указана'}
          </Alert>
        ) : null}

        {state?.state === 'done' && state.report ? (
          <>
            <Report report={state.report} />
            <Group>
              <Badge variant="light" color="gray">
                Прогон завершён
              </Badge>
              <Button variant="default" loading={reset.isPending} onClick={() => reset.mutate()}>
                Сбросить
              </Button>
            </Group>
          </>
        ) : null}
      </Stack>
    </Paper>
  );
}
