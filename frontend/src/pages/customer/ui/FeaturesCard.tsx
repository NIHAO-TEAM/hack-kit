import { Card, Stack, Table, Text, Title } from '@mantine/core';
import { byContribution, formulaText, type CustomerFeature } from '@entities/customer-detail';
import { featureTitle } from '@entities/model-config';

const num = (n: number) => String(Math.round(n * 100) / 100).replace('.', ',');

export function FeaturesCard({
  features,
  score,
}: {
  features: readonly CustomerFeature[];
  score: number | null;
}) {
  return (
    <Card withBorder radius="sm" padding="md">
      <Stack gap="xs">
        <Title order={2} size="h4">
          Признаки риска клиента
        </Title>
        {features.length === 0 ? <Text c="dimmed">Признаков риска нет.</Text> : null}
        {features.length > 0 ? (
          <Table aria-label="Признаки риска клиента">
            <Table.Thead>
              <Table.Tr>
                <Table.Th>Признак</Table.Th>
                <Table.Th>Значение</Table.Th>
                <Table.Th>Вес</Table.Th>
                <Table.Th>Вклад</Table.Th>
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {byContribution(features).map((f) => (
                <Table.Tr key={f.id}>
                  <Table.Td>
                    <Text span fw={500}>
                      {featureTitle(f.id)}
                    </Text>{' '}
                    <Text span size="xs" c="dimmed">
                      {f.id}
                    </Text>
                  </Table.Td>
                  <Table.Td>{num(f.value)}</Table.Td>
                  <Table.Td>{num(f.weight)}</Table.Td>
                  <Table.Td>{num(f.contribution)}</Table.Td>
                </Table.Tr>
              ))}
            </Table.Tbody>
          </Table>
        ) : null}
        {score != null ? (
          <Text size="sm" ff="monospace">
            {formulaText(features, score)}
          </Text>
        ) : null}
      </Stack>
    </Card>
  );
}
