import { Alert, Stack, Table, Text } from '@mantine/core';
import { byContribution, type CustomerFeature, type ServiceRisk } from '@entities/customer-detail';
import { featureTitle } from '@entities/model-config';
import { Money } from '@shared/ui';

const num = (n: number) => String(Math.round(n * 100) / 100).replace('.', ',');

/** Оценка одной живой услуги: P, R, P·R, признаки с весом и значением, проверки качества. */
export function ServiceRiskPanel({ risk }: { risk: ServiceRisk }) {
  const features: readonly CustomerFeature[] = risk.features;
  return (
    <Stack gap="xs">
      <Text size="sm">
        Вероятность непродления P = {num(risk.probability)} ({Math.round(risk.probability * 100)} %)
        · сумма под угрозой R = <Money cents={risk.rCents} /> · ожидаемая потеря P·R ={' '}
        <Money cents={risk.lossCents} />
      </Text>
      {features.length === 0 ? <Text c="dimmed">Признаков риска по услуге нет.</Text> : null}
      {features.length > 0 ? (
        <Table aria-label="Признаки услуги">
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
      {risk.quality.length > 0 ? (
        <Alert color="yellow" variant="light" title="Проверки качества данных">
          {risk.quality.map((q) => (
            <Text size="sm" key={q}>
              {q}
            </Text>
          ))}
        </Alert>
      ) : null}
    </Stack>
  );
}
