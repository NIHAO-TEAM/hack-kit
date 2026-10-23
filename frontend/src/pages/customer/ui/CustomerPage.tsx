import { Anchor, Badge, Card, Grid, Group, Stack, Text, Title } from '@mantine/core';
import { useQuery } from '@tanstack/react-query';
import { Link } from '@tanstack/react-router';
import { customerQuery, type CustomerDetail } from '@entities/customer-detail';
import { ApiError } from '@shared/api';
import { formatMoney } from '@shared/lib';
import { LevelBadge, Money, StateEmpty, StateError, StateLoading, StateOffline } from '@shared/ui';
import { ActionsCard } from './ActionsCard';
import { FeaturesCard } from './FeaturesCard';
import { ServiceList } from './ServiceList';

function BackLink() {
  return (
    <Anchor component={Link} to="/queue" size="sm">
      ← Очередь
    </Anchor>
  );
}

/** Скор в шапке: «нет оценки» и «потерян» вместо числа, с причиной. */
function scoreText(customer: CustomerDetail['customer']): string {
  if (customer.level === 'lost') return 'потерян: все услуги заблокированы';
  if (customer.score == null) return 'нет оценки';
  return `скор ${Math.round(customer.score * 100)} %`;
}

function NotFound({ id }: { id: number }) {
  return (
    <StateEmpty title="Такого клиента нет" action={<BackLink />}>
      Клиент {Number.isNaN(id) ? 'с таким номером' : id} не найден: проверьте номер или вернитесь к
      очереди.
    </StateEmpty>
  );
}

export function CustomerPage({ id }: { id: number }) {
  const valid = Number.isInteger(id);
  const query = useQuery({ ...customerQuery(id), enabled: valid });
  const error = query.error instanceof ApiError ? query.error : null;

  if (!valid) return <NotFound id={id} />;
  if (query.isPending) return <StateLoading label="Загрузка карточки клиента" />;
  if (!query.data) {
    if (error?.status === 404) {
      return <NotFound id={id} />;
    }
    return error?.isNetwork ? (
      <StateOffline onRetry={() => void query.refetch()} />
    ) : (
      <StateError
        title="Не удалось загрузить карточку"
        requestId={error?.requestId}
        onRetry={() => void query.refetch()}
      >
        {error?.message}
      </StateError>
    );
  }

  const { customer, account, services, features, actions, explanation } = query.data;
  const lost = customer.level === 'lost';

  return (
    <Stack>
      <BackLink />
      {query.isError ? <StateOffline onRetry={() => void query.refetch()} /> : null}
      <Card withBorder radius="sm" padding="md" component="header">
        <Group justify="space-between" align="flex-start" wrap="wrap">
          <Stack gap={4}>
            <Title order={1} size="h2">
              Клиент {customer.id}
            </Title>
            <Text c="dimmed">{customer.email}</Text>
            <Text size="sm">
              Баланс {formatMoney(account?.balance ?? customer.balance)} · пополнено{' '}
              {formatMoney(account?.totalReplenished ?? customer.totalReplenished)}
            </Text>
          </Stack>
          <Stack gap={4} align="flex-end">
            <Group gap="xs">
              <LevelBadge level={customer.level} />
              <Text size="sm" c="dimmed">
                {scoreText(customer)}
              </Text>
            </Group>
            <Text size="sm" c="dimmed">
              {lost ? 'Уже потеряно' : 'Ожидаемая потеря'}
            </Text>
            <Text fz="xl" fw={600}>
              <Money cents={customer.expectedLossCents} tone={lost ? 'lost' : 'default'} />
            </Text>
          </Stack>
        </Group>
      </Card>
      <Grid>
        <Grid.Col span={{ base: 12, lg: 8 }}>
          <Stack>
            {lost ? null : <ActionsCard actions={actions} />}
            <ServiceList services={services} />
            <FeaturesCard features={features} score={customer.score ?? null} />
          </Stack>
        </Grid.Col>
        <Grid.Col span={{ base: 12, lg: 4 }}>
          <Card withBorder radius="sm" padding="md">
            <Stack gap="xs">
              <Title order={2} size="h4">
                Откуда сумма
              </Title>
              <Text size="sm">
                {explanation ??
                  'Платежи на 30 дней × вероятность срыва; не входят новые клиенты, скидки, возвраты.'}
              </Text>
              <Text size="sm" c="dimmed">
                Не входят новые клиенты, скидки и возвраты.
              </Text>
              {customer.level === 'unscored' ? (
                <Badge variant="outline">Оценки нет: данных пока мало</Badge>
              ) : null}
            </Stack>
          </Card>
        </Grid.Col>
      </Grid>
    </Stack>
  );
}
