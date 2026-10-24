import { Anchor, Card, Group, Stack, Text, Title } from '@mantine/core';
import { Link } from '@tanstack/react-router';
import type { Summary } from '../api/analytics';
import { highWithoutTaskText, inWorkText } from '../model/retention';

/** «Работа с клиентами»: какая часть потери уже у маркетологов и кого никто не взял. */
export function RetentionBlock({ summary }: { summary: Summary }) {
  const r = summary.retention;
  if (!r) return null;
  return (
    <Card withBorder aria-label="Работа с клиентами">
      <Stack gap="xs">
        <Title order={2} size="h4">
          Работа с клиентами
        </Title>
        <Text>{inWorkText(r.customersInWork, r.lossInWorkCents, summary.expectedLossCents)}</Text>
        <Group gap="lg">
          <Anchor component={Link} to="/tasks" search={{ filter: 'overdue' }}>
            Просрочено: {r.tasksOverdue}
          </Anchor>
          <Anchor component={Link} to="/queue" search={{ task: 'none', level: 'high' }}>
            {highWithoutTaskText(r.highWithoutTask, r.highWithoutTaskLossCents)}
          </Anchor>
        </Group>
      </Stack>
    </Card>
  );
}
