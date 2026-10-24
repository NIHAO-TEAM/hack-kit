import { Accordion, Alert, Group, Stack, Text, Title } from '@mantine/core';
import { lossSum, type CustomerDetail, type ServiceRisk } from '@entities/customer-detail';
import { Money, RescueWindow } from '@shared/ui';
import { ServiceRiskPanel } from './ServiceRiskPanel';

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

/** Услуга живая, если сервер её оценивает: удалённая и заблокированная не продлеваются. */
const isDead = (s: Service) => s.status === 'deleted' || s.status === 'blocked';

/** Услуги клиента: срок, статус, окно спасения; раскрытие — срок и оценка услуги с сервера, под списком сверка суммы с шапкой. */
export function ServiceList({
  services,
  risks = [],
  expectedLossCents = null,
  asOf,
  asOfFailed = false,
}: {
  services: readonly Service[];
  /** Оценки живых услуг (`serviceRisks`); пусто — сервер их не отдал. */
  risks?: readonly ServiceRisk[];
  /** Ожидаемая потеря клиента из шапки для сверки; `null` — сверять нечего. */
  expectedLossCents?: number | null;
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
              <Stack gap="xs">
                <Text size="sm" c="dimmed">
                  Срок: {new Date(s.expiresAt).toLocaleDateString('ru-RU')}.
                </Text>
                <ServiceEstimate service={s} risk={risks.find((r) => r.serviceId === s.id)} />
              </Stack>
            </Accordion.Panel>
          </Accordion.Item>
        ))}
      </Accordion>
      {risks.length > 0 && expectedLossCents != null ? (
        <Reconciliation sum={lossSum(risks)} expectedLossCents={expectedLossCents} />
      ) : null}
    </Stack>
  );
}

function ServiceEstimate({ service, risk }: { service: Service; risk: ServiceRisk | undefined }) {
  if (isDead(service)) {
    return (
      <Text size="sm" c="dimmed">
        Не продлена, в ожидаемую потерю не входит.
      </Text>
    );
  }
  if (!risk)
    return (
      <Text size="sm" c="dimmed">
        Оценок по услугам нет.
      </Text>
    );
  return <ServiceRiskPanel risk={risk} />;
}

/** Сумма потерь по услугам должна сходиться с числом в шапке; расхождение больше цента не прячем. */
function Reconciliation({ sum, expectedLossCents }: { sum: number; expectedLossCents: number }) {
  if (Math.abs(sum - expectedLossCents) > 1) {
    return (
      <Alert color="red" variant="light" title="Сумма не сходится с шапкой">
        Сумма потерь по услугам <Money cents={sum} /> ≠ ожидаемая потеря клиента{' '}
        <Money cents={expectedLossCents} />.
      </Alert>
    );
  }
  return (
    <Text size="sm">
      Сумма потерь по услугам = ожидаемая потеря клиента: <Money cents={sum} />
    </Text>
  );
}
