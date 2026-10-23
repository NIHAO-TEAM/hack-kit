import { Group, Paper, Stack, Text, Title } from '@mantine/core';
import { deletionAt, inWaitWindow, type CustomerRow } from '@entities/customer';
import { Countdown } from '@shared/ui';
import classes from './DeletionStrip.module.css';

/** Полоса «удалит биллинг в ближайшие 5 дней»: клиенты, у которых идёт ожидание, ближайшие сверху. */
export function DeletionStrip({
  rows,
  asOf,
  onOpen,
}: {
  rows: CustomerRow[];
  asOf: string;
  onOpen: (id: number) => void;
}) {
  const urgent = rows
    .filter((r) => inWaitWindow(r, asOf))
    .sort((a, b) => deletionAt(a) - deletionAt(b));
  if (urgent.length === 0) return null;
  return (
    <Paper withBorder p="lg" radius="lg" component="section" aria-label="Удалит биллинг">
      <Stack gap="sm">
        <Title order={2} size="h4">
          Удалит биллинг в ближайшие 5 дней: {urgent.length}
        </Title>
        <Group gap="sm">
          {urgent.map((r) => (
            <button key={r.id} type="button" onClick={() => onOpen(r.id)} className={classes.chip}>
              <Text span inherit fw={600}>
                {r.email}
              </Text>
              {' · '}
              <Countdown until={new Date(deletionAt(r)).toISOString()} asOf={asOf} />
            </button>
          ))}
        </Group>
      </Stack>
    </Paper>
  );
}
