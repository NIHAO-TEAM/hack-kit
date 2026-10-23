import { Accordion, Group, Stack, Table, Text, Title } from '@mantine/core';
import {
  byContribution,
  formulaText,
  type CustomerDetail,
  type CustomerFeature,
} from '@entities/customer-detail';
import { featureTitle } from '@entities/model-config';
import { RescueWindow } from '@shared/ui';

type Service = CustomerDetail['services'][number];

const statusLabel: Record<Service['status'], string> = {
  active: 'работает',
  blocked: 'заблокирована',
  deleted: 'удалена',
  suspended: 'приостановлена',
  prolong_wait: 'ждёт продления',
};

const termLabel: Record<Service['paymentTerm'], string> = {
  hour: 'час',
  week: 'неделя',
  month: 'месяц',
  quarter_year: 'квартал',
  half_year: 'полгода',
  year: 'год',
};

const num = (n: number) => String(Math.round(n * 100) / 100).replace('.', ',');

function Features({
  features,
  score,
}: {
  features: readonly CustomerFeature[];
  score: number | null;
}) {
  if (features.length === 0) return <Text c="dimmed">Признаков риска нет.</Text>;
  return (
    <Stack gap="xs">
      <Table aria-label="Признаки риска">
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
      {score != null ? (
        <Text size="sm" ff="monospace">
          {formulaText(features, score)}
        </Text>
      ) : null}
    </Stack>
  );
}

/** Услуги клиента: срок, статус, окно спасения; раскрытие — признаки и формула. */
export function ServiceList({
  services,
  features,
  score,
  asOf = new Date().toISOString(),
}: {
  services: readonly Service[];
  features: readonly CustomerFeature[];
  score: number | null;
  /** Время стенда; пока нет `/api/clock` — часы браузера [ПРЕДПОЛОЖЕНО]. */
  asOf?: string;
}) {
  return (
    <Stack gap="xs">
      <Title order={2} size="h4">
        Услуги
      </Title>
      {services.length === 0 ? <Text c="dimmed">У клиента нет услуг.</Text> : null}
      <Accordion variant="separated" radius="sm">
        {services.map((s) => (
          <Accordion.Item key={s.id} value={String(s.id)}>
            <Accordion.Control aria-label={`Услуга ${s.id}`}>
              <Group justify="space-between" wrap="wrap">
                <Stack gap={0}>
                  <Text fw={500}>{s.name}</Text>
                  <Text size="sm" c="dimmed">
                    {termLabel[s.paymentTerm]} · {statusLabel[s.status]}
                  </Text>
                </Stack>
                <RescueWindow expiresAt={s.status === 'deleted' ? null : s.expiresAt} asOf={asOf} />
              </Group>
            </Accordion.Control>
            <Accordion.Panel>
              <Features features={features} score={score} />
            </Accordion.Panel>
          </Accordion.Item>
        ))}
      </Accordion>
    </Stack>
  );
}
