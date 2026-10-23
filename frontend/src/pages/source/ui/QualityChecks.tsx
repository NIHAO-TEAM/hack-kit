import { Group, Paper, Stack, Text, Title } from '@mantine/core';
import { useQuery } from '@tanstack/react-query';
import { qualityQuery } from '../api/source';
import { qualityTitle } from '../model/quality';
import { StateError, StateLoading } from '@shared/ui';

/** Проверки качества: сколько услуг и клиентов отметила каждая. Нули не прячем: «0» — тоже ответ. */
export function QualityChecks() {
  const query = useQuery(qualityQuery);
  if (query.isPending) return <StateLoading label="Загрузка проверок качества" />;
  if (query.isError) {
    return (
      <StateError title="Не удалось загрузить проверки" onRetry={() => void query.refetch()}>
        {query.error.message}
      </StateError>
    );
  }
  const entries = Object.entries(query.data).sort(([a], [b]) => a.localeCompare(b));
  return (
    <Paper withBorder p="lg" radius="lg" component="section" aria-label="Проверки качества данных">
      <Stack gap="sm">
        <Title order={2} size="h4">
          Можно ли верить данным
        </Title>
        {entries.length === 0 ? (
          <Text c="dimmed">Проверки ничего не нашли: ошибок данных нет.</Text>
        ) : (
          entries.map(([id, count]) => (
            <Group key={id} justify="space-between" wrap="nowrap">
              <Text>{qualityTitle(id)}</Text>
              <Text fw={600}>{count}</Text>
            </Group>
          ))
        )}
      </Stack>
    </Paper>
  );
}
