import { Stack, Title } from '@mantine/core';
import { StateEmpty } from '@shared/ui';

export function SummaryPage() {
  return (
    <Stack>
      <Title order={1}>Сводка</Title>
      <StateEmpty title="Экран появится позже">
        Сводка риска и ожидаемых потерь появится на следующем этапе.
      </StateEmpty>
    </Stack>
  );
}
