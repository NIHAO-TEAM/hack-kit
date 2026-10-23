import { Stack, Title } from '@mantine/core';
import { StateEmpty } from '@shared/ui';

export function SourcePage() {
  return (
    <Stack>
      <Title order={1}>Источник данных</Title>
      <StateEmpty title="Экран появится позже">
        Состояние источника данных появится на следующем этапе.
      </StateEmpty>
    </Stack>
  );
}
