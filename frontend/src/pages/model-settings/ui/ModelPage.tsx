import { Stack, Title } from '@mantine/core';
import { StateEmpty } from '@shared/ui';

export function ModelPage() {
  return (
    <Stack>
      <Title order={1}>Модель</Title>
      <StateEmpty title="Экран появится позже">
        Настройки модели появятся на следующем этапе.
      </StateEmpty>
    </Stack>
  );
}
