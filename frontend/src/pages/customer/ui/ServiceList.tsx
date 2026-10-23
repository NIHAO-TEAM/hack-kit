import { Accordion, Group, Stack, Text, Title } from '@mantine/core';
import type { CustomerDetail } from '@entities/customer-detail';
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

/** Услуги клиента: срок, статус, окно спасения; раскрытие — срок и пояснение (признаки по услугам сервер пока не отдаёт). */
export function ServiceList({
  services,
  asOf,
  asOfFailed = false,
}: {
  services: readonly Service[];
  /** Время модели из сводки (`asOf`); пока оно не загружено — `null`. */
  asOf: string | null;
  /** Сводка не загрузилась: вместо «загружается» — «недоступно». */
  asOfFailed?: boolean;
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
            <Accordion.Control>
              <Group justify="space-between" wrap="wrap">
                <Stack gap={0}>
                  <Text fw={500}>{s.name}</Text>
                  <Text size="sm" c="dimmed">
                    {termLabel[s.paymentTerm]} · {statusLabel[s.status]}
                  </Text>
                </Stack>
                {asOf ? (
                  <RescueWindow
                    expiresAt={s.status === 'deleted' ? null : s.expiresAt}
                    asOf={asOf}
                  />
                ) : (
                  <Text size="sm" c="dimmed">
                    {asOfFailed ? 'время модели недоступно' : 'время модели загружается'}
                  </Text>
                )}
              </Group>
            </Accordion.Control>
            <Accordion.Panel>
              <Text size="sm" c="dimmed">
                Срок: {new Date(s.expiresAt).toLocaleDateString('ru-RU')}. Признаки и вероятность по
                услуге появятся, когда сервер начнёт отдавать их по услугам.
              </Text>
            </Accordion.Panel>
          </Accordion.Item>
        ))}
      </Accordion>
    </Stack>
  );
}
