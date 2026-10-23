import { Alert, Code, List, Stack, Table, Text, Title } from '@mantine/core';
import { useQuery } from '@tanstack/react-query';
import { ApiError } from '@shared/api';
import { modelSettingsOptions } from '../api/settings';
import { defaultThresholds, features, formatWeight, weightOf } from '../model/features';
import { StateError, StateLoading, StateOffline } from '@shared/ui';

/** Пример клиента 9 набора организаторов: числа посчитаны оракулом (docs/model.md). */
const example = [
  { id: 'funds', f: '0,366697' },
  { id: 'flight', f: '0,605337' },
  { id: 'susp', f: '1' },
  { id: 'fail', f: '1' },
  { id: 'noauto', f: '1' },
] as const;

export function AboutModelPage() {
  const query = useQuery(modelSettingsOptions);

  if (query.isPending) return <StateLoading label="Загрузка описания модели" />;

  if (query.isError && !query.data) {
    const error = query.error instanceof ApiError ? query.error : null;
    return (
      <StateError
        title={error?.isNetwork ? 'Нет связи с сервером' : 'Не удалось загрузить настройки модели'}
        requestId={error?.requestId}
        onRetry={() => void query.refetch()}
      >
        Описание модели зависит от текущих весов, без них показывать нечего.
      </StateError>
    );
  }

  const settings = query.data;
  const weights = settings?.weights;
  const high = settings?.thresholds.high ?? defaultThresholds.high;
  const medium = settings?.thresholds.medium ?? defaultThresholds.medium;
  const horizon = settings?.horizonDays ?? 30;

  return (
    <Stack gap="lg" maw={860}>
      {query.isError ? <StateOffline onRetry={() => void query.refetch()} /> : null}
      <Title order={1}>О модели</Title>
      <Text>
        У каждой живой услуги клиента мы считаем вероятность ухода. Скор клиента — вероятность
        худшей услуги. Умножаем вероятность на деньги, которые услуга принесёт за {horizon} дн., и
        получаем ожидаемую потерю в евро: по ней строится очередь. Каждая причина в карточке клиента
        — сработавший признак.
      </Text>

      <Stack gap="xs">
        <Title order={2} size="h3">
          Признаки и веса
        </Title>
        <Table.ScrollContainer minWidth={560}>
          <Table>
            <Table.Thead>
              <Table.Tr>
                <Table.Th>Признак</Table.Th>
                <Table.Th>Смысл</Table.Th>
                <Table.Th ta="right">Вес</Table.Th>
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {features.map((f) => (
                <Table.Tr key={f.id}>
                  <Table.Td>{f.title}</Table.Td>
                  <Table.Td>{f.meaning}</Table.Td>
                  <Table.Td ta="right" style={{ fontVariantNumeric: 'tabular-nums' }}>
                    {formatWeight(weightOf(f, weights))}
                  </Table.Td>
                </Table.Tr>
              ))}
            </Table.Tbody>
          </Table>
        </Table.ScrollContainer>
        <Text size="sm" c="dimmed">
          Веса — экспертная оценка команды, подтверждения эксперта aéza по ним нет. Их меняет
          аналитик на экране «Модель», очередь пересчитывается сразу.
        </Text>
      </Stack>

      <Stack gap="xs">
        <Title order={2} size="h3">
          Формула
        </Title>
        <Code block>
          {`P услуги = 1 − Π (1 − вес · признак)
скор клиента = 100 · max P по живым услугам
ожидаемая потеря = Σ round(P · деньги услуги)`}
        </Code>
        <Text size="sm">
          Признаки складываются как независимые причины: результат не превышает 100, а один сильный
          признак не гасится слабыми.
        </Text>
      </Stack>

      <Stack gap="xs">
        <Title order={2} size="h3">
          Уровни
        </Title>
        <List>
          <List.Item>Высокий — скор от {high}: действовать сегодня.</List.Item>
          <List.Item>Средний — скор от {medium}: в очередь на ближайшие дни.</List.Item>
          <List.Item>Низкий — ниже {medium}: наблюдать.</List.Item>
          <List.Item>Потерян — живых услуг нет, скора нет, считается «уже потеряно».</List.Item>
        </List>
      </Stack>

      <Stack gap="xs">
        <Title order={2} size="h3">
          Пример: клиент 9
        </Title>
        <Text>У услуги 9 сработали пять признаков. Ниже — значение признака и вклад в итог.</Text>
        <Table.ScrollContainer minWidth={420}>
          <Table>
            <Table.Thead>
              <Table.Tr>
                <Table.Th>Признак</Table.Th>
                <Table.Th ta="right">Значение</Table.Th>
                <Table.Th ta="right">Вес</Table.Th>
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {example.map((row) => {
                const feature = features.find((f) => f.id === row.id)!;
                return (
                  <Table.Tr key={row.id}>
                    <Table.Td>{feature.title}</Table.Td>
                    <Table.Td ta="right" style={{ fontVariantNumeric: 'tabular-nums' }}>
                      {row.f}
                    </Table.Td>
                    <Table.Td ta="right" style={{ fontVariantNumeric: 'tabular-nums' }}>
                      {formatWeight(weightOf(feature, weights))}
                    </Table.Td>
                  </Table.Tr>
                );
              })}
            </Table.Tbody>
          </Table>
        </Table.ScrollContainer>
        <Text>
          При весах по умолчанию вероятность услуги 9 — 88,14 %, скор клиента — 88,14 (высокий
          уровень), ожидаемая потеря клиента — 3 459,27 €.
        </Text>
        <Alert variant="light" color="gray">
          Пример посчитан эталонным расчётом (<Code>tools/oracle</Code>) при весах по умолчанию.
          Если аналитик изменил веса, числа примера останутся прежними, а таблица весов покажет
          текущие.
        </Alert>
      </Stack>
    </Stack>
  );
}
