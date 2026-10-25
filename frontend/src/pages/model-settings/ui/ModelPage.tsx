import { Badge, Group, Stack, Text, Title } from '@mantine/core';
import { useQuery } from '@tanstack/react-query';
import { settingsQuery } from '@entities/model-config';
import { ApiError } from '@shared/api';
import { StateError, StateLoading, StateOffline } from '@shared/ui';
import { SettingsForm } from './SettingsForm';

export function ModelPage({ editable = true }: { editable?: boolean }) {
  const query = useQuery(settingsQuery);
  const error = query.error instanceof ApiError ? query.error : null;

  if (query.isPending) return <StateLoading label="Загрузка настроек модели" />;
  if (!query.data) {
    return error?.isNetwork ? (
      <StateOffline onRetry={() => void query.refetch()} />
    ) : (
      <StateError
        title="Не удалось загрузить настройки"
        requestId={error?.requestId}
        onRetry={() => void query.refetch()}
      >
        {error?.message}
      </StateError>
    );
  }

  return (
    <Stack>
      <Group justify="space-between" align="baseline">
        <Title order={1}>Модель</Title>
        <Group gap="xs">
          <Badge variant="light" aria-label="Версия настроек">
            Версия {query.data.version}
          </Badge>
          <Text size="sm" c="dimmed">
            Изменения применяются ко всей очереди
          </Text>
        </Group>
      </Group>
      {query.isError ? <StateOffline onRetry={() => void query.refetch()} /> : null}
      <SettingsForm key={query.data.version} settings={query.data} readOnly={!editable} />
    </Stack>
  );
}
