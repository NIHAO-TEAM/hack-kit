import { Anchor, Card, Group, List, Progress, Stack, Text, Title } from '@mantine/core';
import { Link } from '@tanstack/react-router';
import {
  activeReasons,
  riskReasons,
  type CustomerDetail,
  type CustomerFeature,
  type ReasonStrength,
  type RiskReason,
  type ServiceRisk,
} from '@entities/customer-detail';
import { featureTitle } from '@entities/model-config';
import { Money } from '@shared/ui';

type Service = CustomerDetail['services'][number];

const strengthLabel: Record<ReasonStrength, string> = {
  strong: 'сильная',
  noticeable: 'заметная',
  weak: 'слабая',
};

function ReasonList({ reasons }: { reasons: readonly RiskReason[] }) {
  return (
    <List size="sm" spacing={2}>
      {reasons.map((r) => (
        <List.Item key={r.id}>
          {r.text}{' '}
          <Text span size="xs" c="dimmed">
            {strengthLabel[r.strength]}
          </Text>
        </List.Item>
      ))}
    </List>
  );
}

/** Причины риска по услугам словами: сумма под угрозой, полоса доли и что именно плохо. Весов и формул нет, проверка до цента — на «О модели». */
export function RiskReasonsCard({
  services,
  risks,
  features,
}: {
  services: readonly Service[];
  risks: readonly ServiceRisk[];
  features: readonly CustomerFeature[];
}) {
  const byService = riskReasons(risks, featureTitle);
  const clientReasons = activeReasons(features, featureTitle);
  const empty = byService.length === 0 && clientReasons.length === 0;
  return (
    <Card withBorder radius="sm" padding="md">
      <Stack gap="sm">
        <Title order={2} size="h4">
          Причины риска
        </Title>
        {empty ? <Text c="dimmed">Явных причин риска нет</Text> : null}
        {byService.map((s) => (
          <Stack key={s.serviceId} gap={4} data-testid="risk-service">
            <Group justify="space-between" wrap="wrap" gap="xs">
              <Text fw={500}>
                {services.find((x) => x.id === s.serviceId)?.name ?? `Услуга ${s.serviceId}`}
              </Text>
              <Text size="sm">
                <Money cents={s.lossCents} /> под угрозой
              </Text>
            </Group>
            <Progress value={s.share * 100} size="sm" aria-label="Доля услуги в потере клиента" />
            {s.reasons.length > 0 ? (
              <ReasonList reasons={s.reasons} />
            ) : (
              <Text size="sm" c="dimmed">
                Явных причин риска нет
              </Text>
            )}
          </Stack>
        ))}
        {byService.length === 0 && clientReasons.length > 0 ? (
          <ReasonList reasons={clientReasons} />
        ) : null}
        <Anchor component={Link} to="/about" size="sm">
          Как считается риск
        </Anchor>
      </Stack>
    </Card>
  );
}
