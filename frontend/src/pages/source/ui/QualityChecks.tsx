import { Anchor, Group, List, Paper, Stack, Text, Title } from '@mantine/core';
import { type SourceStatus } from '../api/source';

type Check = SourceStatus['checks'][number];

/** Проверки качества данных из статуса источника: подписи и услуги приходят с сервера. */
export function QualityChecks({ checks }: { checks: Check[] }) {
  const found = checks.filter((c) => !c.ok);
  return (
    <Paper withBorder p="lg" radius="lg" component="section" aria-label="Проверки качества данных">
      <Stack gap="sm">
        <Title order={2} size="h4">
          Можно ли верить данным
        </Title>
        {checks.length === 0 ? (
          <Text c="dimmed">Проверки считаются: результат появится после пересчёта.</Text>
        ) : (
          <>
            {found.length === 0 ? <Text c="dimmed">Противоречий не найдено.</Text> : null}
            {checks.map((c) => (
              <Stack key={c.id} gap={4}>
                <Group gap="xs" wrap="nowrap">
                  <Text fw={c.ok ? 400 : 600} c={c.ok ? 'dimmed' : undefined}>
                    {c.message ?? c.id}
                  </Text>
                </Group>
                {c.items?.length ? (
                  <List size="sm" spacing={2} aria-label={`Услуги: ${c.message ?? c.id}`}>
                    {c.items.map((it) => (
                      <List.Item key={it.serviceId}>
                        Услуга {it.serviceId}, клиент{' '}
                        <Anchor href={`/queue/${it.accountId}`}>{it.accountId}</Anchor>
                      </List.Item>
                    ))}
                  </List>
                ) : null}
              </Stack>
            ))}
          </>
        )}
      </Stack>
    </Paper>
  );
}
