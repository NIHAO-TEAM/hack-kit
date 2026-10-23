import { Badge, Card, Group, List, Stack, Text, Title } from '@mantine/core';
import type { CustomerAction } from '@entities/customer-detail';
import { Changed, Money } from '@shared/ui';

/** Главное действие с ценой — крупно; остальные — списком. Применение — в эмуляторе (этап 5). */
export function ActionsCard({
  actions,
  changed = false,
}: {
  actions: readonly CustomerAction[];
  /** Действия изменились после пересчёта: содержимое карточки подсвечено. */
  changed?: boolean;
}) {
  const [main, ...rest] = actions;
  if (!main) {
    return (
      <Card withBorder radius="sm" padding="md">
        <Text c="dimmed">Действий для этого клиента нет.</Text>
      </Card>
    );
  }
  return (
    <Card withBorder radius="sm" padding="md">
      <Changed active={changed}>
        <Stack gap="sm">
          <Title order={2} size="h4">
            Что сделать
          </Title>
          <Group gap="xs" align="baseline">
            <Text fz="xl" fw={600} c="var(--text-strong)">
              {main.title}
            </Text>
            <Text fz="xl">
              · <Money cents={main.effectCents} tone="effect" />
            </Text>
            {main.id === 'billing' ? <Badge variant="outline">к биллингу</Badge> : null}
          </Group>
          {rest.length > 0 ? (
            <List spacing={4} aria-label="Другие действия">
              {rest.map((a) => (
                <List.Item key={a.id}>
                  {a.title} · <Money cents={a.effectCents} tone="effect" />
                  {a.id === 'billing' ? ' · к биллингу' : ''}
                </List.Item>
              ))}
            </List>
          ) : null}
        </Stack>
      </Changed>
    </Card>
  );
}
