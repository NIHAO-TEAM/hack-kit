import { Stack, Title } from '@mantine/core';
import { StateEmpty } from '@shared/ui';

export function QueuePage() {
  return (
    <Stack>
      <Title order={1}>Очередь</Title>
      <StateEmpty title="Экран появится позже">
        Список клиентов под риском оттока появится на следующем этапе.
      </StateEmpty>
    </Stack>
  );
}
