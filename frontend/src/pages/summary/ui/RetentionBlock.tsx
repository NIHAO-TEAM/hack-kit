import { Anchor, Card, Group, Stack, Text, Title } from '@mantine/core';
import { useRouter } from '@tanstack/react-router';
import type { MouseEvent, ReactNode } from 'react';
import type { Summary } from '../api/analytics';
import { highWithoutTaskText, inWorkText } from '../model/retention';

/** Обычный href (новая вкладка работает), клик — без перезагрузки. */
function InternalLink({ href, children }: { href: string; children: ReactNode }) {
  const router = useRouter();
  const open = (e: MouseEvent<HTMLAnchorElement>) => {
    if (e.metaKey || e.ctrlKey || e.shiftKey || e.button !== 0) return;
    e.preventDefault();
    void router.navigate({ href });
  };
  return (
    <Anchor href={href} onClick={open}>
      {children}
    </Anchor>
  );
}

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
          <InternalLink href="/tasks?filter=overdue">Просрочено: {r.tasksOverdue}</InternalLink>
          <InternalLink href="/queue?task=none&level=high">
            {highWithoutTaskText(r.highWithoutTask, r.highWithoutTaskLossCents)}
          </InternalLink>
        </Group>
      </Stack>
    </Card>
  );
}
