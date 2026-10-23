import { Paper, SimpleGrid, Skeleton, Stack, Text } from '@mantine/core';
import { Money } from '@shared/ui';

interface Card {
  title: string;
  hint: string;
  cents: number | null;
  tone: 'default' | 'lost';
}

/** Три числа над очередью. У каждого подпись «что это значит» — число без смысла не показываем. */
export function SummaryCards({
  expectedLossCents,
  atRiskCents,
  lostCents,
}: {
  expectedLossCents: number | null;
  atRiskCents: number | null;
  lostCents: number | null;
}) {
  const cards: Card[] = [
    {
      title: 'Ожидаемая потеря за 30 дней',
      hint: 'Платежи на 30 дней × вероятность срыва. Новые клиенты, скидки и возвраты не входят.',
      cents: expectedLossCents,
      tone: 'default',
    },
    {
      title: 'Под угрозой',
      hint: 'Деньги клиентов, у которых уровень риска высокий или средний.',
      cents: atRiskCents,
      tone: 'default',
    },
    {
      title: 'Уже потеряно',
      hint: 'Услуги, которые биллинг удалил: вернуть их нельзя.',
      cents: lostCents,
      tone: 'lost',
    },
  ];
  return (
    <SimpleGrid cols={{ base: 1, md: 3 }} spacing="lg">
      {cards.map((c) => (
        <Paper key={c.title} withBorder p="lg" radius="lg" component="section" aria-label={c.title}>
          <Stack gap={4}>
            <Text size="sm" c="dimmed">
              {c.title}
            </Text>
            {c.cents == null ? (
              <Skeleton height={36} width="60%" />
            ) : (
              <Text fz={32} fw={700} lh={1.2} component="div">
                <Money cents={c.cents} tone={c.tone} />
              </Text>
            )}
            <Text size="xs" c="dimmed">
              {c.hint}
            </Text>
          </Stack>
        </Paper>
      ))}
    </SimpleGrid>
  );
}
