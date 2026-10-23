import { Card, Group, SimpleGrid, Stack, Text, Title } from '@mantine/core';
import { useQuery } from '@tanstack/react-query';
import { churnChartQuery, summaryQuery } from '../api/analytics';
import { ChurnChart } from './ChurnChart';
import { ApiError } from '@shared/api';
import { LevelBadge, Money, StateError, StateLoading, StateOffline } from '@shared/ui';

function failure(error: unknown, title: string, retry: () => void) {
  const e = error instanceof ApiError ? error : null;
  return e?.isNetwork ? (
    <StateOffline onRetry={retry} />
  ) : (
    <StateError title={title} requestId={e?.requestId} onRetry={retry}>
      {e?.message}
    </StateError>
  );
}

export function SummaryPage() {
  const summary = useQuery(summaryQuery);
  const chart = useQuery(churnChartQuery);

  return (
    <Stack>
      <Title order={1}>Сводка</Title>

      {summary.isPending ? (
        <StateLoading label="Загрузка сводки" />
      ) : !summary.data ? (
        failure(summary.error, 'Не удалось загрузить сводку', () => void summary.refetch())
      ) : (
        <>
          <SimpleGrid cols={{ base: 1, sm: 3 }}>
            <Card withBorder>
              <Text size="sm" c="dimmed">
                Ожидаемая потеря за 30 дней
              </Text>
              <Text size="xl" fw={600}>
                <Money cents={summary.data.expectedLossCents} />
              </Text>
            </Card>
            <Card withBorder>
              <Text size="sm" c="dimmed">
                Под угрозой
              </Text>
              <Text size="xl" fw={600}>
                <Money cents={summary.data.atRiskCents} />
              </Text>
            </Card>
            <Card withBorder>
              <Text size="sm" c="dimmed">
                Уже потеряно
              </Text>
              <Text size="xl" fw={600}>
                <Money cents={summary.data.lostCents} tone="lost" />
              </Text>
            </Card>
          </SimpleGrid>
          <Group gap="lg" aria-label="Клиенты по уровням риска">
            {(['high', 'medium', 'low', 'unscored', 'lost'] as const).map((l) => (
              <Group key={l} gap={6}>
                <LevelBadge level={l} />
                <Text fw={600}>{summary.data.levels[l]}</Text>
              </Group>
            ))}
          </Group>
        </>
      )}

      <Card withBorder aria-busy={chart.isPending}>
        <Title order={2} size="h4" mb="sm">
          Отток по неделям: факт и прогноз
        </Title>
        {chart.isPending ? (
          <StateLoading label="Загрузка графика" />
        ) : !chart.data ? (
          failure(chart.error, 'Не удалось загрузить график', () => void chart.refetch())
        ) : chart.data.length === 0 ? (
          <Text c="dimmed">Данных для графика пока нет.</Text>
        ) : (
          <ChurnChart points={chart.data} />
        )}
      </Card>
    </Stack>
  );
}
