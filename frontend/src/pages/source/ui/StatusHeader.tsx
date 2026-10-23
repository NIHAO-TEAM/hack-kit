import { Badge, Group, Paper, Stack, Text } from '@mantine/core';
import { type SourceStatus } from '../api/source';
import { sourceLabel } from '../model/quality';

const dateTime = new Intl.DateTimeFormat('ru-RU', {
  dateStyle: 'medium',
  timeStyle: 'short',
  timeZone: 'UTC',
});

function Fact({ label, children }: { label: string; children: string }) {
  return (
    <Stack gap={2}>
      <Text size="sm" c="dimmed">
        {label}
      </Text>
      <Text fw={500}>{children}</Text>
    </Stack>
  );
}

/** Шапка: версия снимка, на какой момент он, когда загружен, откуда. Время — UTC. */
export function StatusHeader({ status }: { status: SourceStatus }) {
  return (
    <Paper withBorder p="lg" radius="lg" component="section" aria-label="Текущий снимок">
      <Group justify="space-between" align="flex-start" wrap="wrap" gap="lg">
        <Fact label="Версия снимка">
          {status.version > 0 ? String(status.version) : 'не загружен'}
        </Fact>
        <Fact label="Данные на момент (UTC)">
          {status.asOf ? dateTime.format(new Date(status.asOf)) : '—'}
        </Fact>
        <Fact label="Загружен (UTC)">
          {status.version > 0 ? dateTime.format(new Date(status.loadedAt)) : '—'}
        </Fact>
        <Stack gap={2}>
          <Text size="sm" c="dimmed">
            Источник
          </Text>
          <Badge variant="light" size="lg">
            {status.source ? (sourceLabel[status.source] ?? status.source) : '—'}
          </Badge>
        </Stack>
      </Group>
    </Paper>
  );
}
