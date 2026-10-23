import { Card, Loader, Stack, Text } from '@mantine/core';
import { useDebouncedValue } from '@mantine/hooks';
import { useQuery } from '@tanstack/react-query';
import { dryRunText, previewQuery, type ModelSettingsPatch } from '@entities/model-config';
import { ApiError } from '@shared/api';

export const PREVIEW_DELAY_MS = 300;

/** Что изменится, если сохранить: считает сервер (`dryRun`), фронт только показывает. */
export function PreviewPanel({ patch, changed }: { patch: ModelSettingsPatch; changed: boolean }) {
  const [debounced] = useDebouncedValue(patch, PREVIEW_DELAY_MS);
  const settled = JSON.stringify(debounced) === JSON.stringify(patch);
  const query = useQuery({ ...previewQuery(debounced), enabled: changed });
  const error = query.error instanceof ApiError ? query.error : null;

  let body;
  if (!changed)
    body = <Text c="dimmed">Сдвиньте вес или порог — здесь появится предпросмотр.</Text>;
  else if (!settled || query.isFetching) {
    body = <Loader size="sm" aria-label="Считаем предпросмотр" />;
  } else if (error) {
    body = (
      <Text c="red" role="alert">
        {error.status === 422 ? 'Проверьте значения полей' : 'Предпросмотр недоступен'}
      </Text>
    );
  } else if (query.data) {
    body = <Text role="status">{dryRunText(query.data)}</Text>;
  }

  return (
    <Card withBorder aria-label="Предпросмотр">
      <Stack gap="xs">
        <Text fw={500}>Что изменится</Text>
        {body}
      </Stack>
    </Card>
  );
}
