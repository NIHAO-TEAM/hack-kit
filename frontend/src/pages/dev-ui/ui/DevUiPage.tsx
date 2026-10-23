import { Divider, Stack, Title } from '@mantine/core';
import { StateEmpty, StateError, StateForbidden, StateLoading, StateOffline } from '@shared/ui';

/** Витрина компонентов: каждое состояние экрана в обеих темах. */
export function DevUiPage() {
  return (
    <Stack gap="lg">
      <Title order={1}>Витрина</Title>
      <Title order={2} size="h4">
        Загрузка
      </Title>
      <StateLoading />
      <Divider />
      <Title order={2} size="h4">
        Пусто
      </Title>
      <StateEmpty title="Клиентов под риском нет">Когда появятся, они будут здесь.</StateEmpty>
      <Divider />
      <Title order={2} size="h4">
        Ошибка
      </Title>
      <StateError title="Не удалось загрузить очередь" requestId="req-42" onRetry={() => undefined}>
        Сервер не ответил за 10 секунд.
      </StateError>
      <Divider />
      <Title order={2} size="h4">
        Нет связи
      </Title>
      <StateOffline onRetry={() => undefined} />
      <Divider />
      <Title order={2} size="h4">
        Нет прав
      </Title>
      <StateForbidden>Ваша роль — маркетолог. Раздел «Модель» доступен аналитику.</StateForbidden>
    </Stack>
  );
}
