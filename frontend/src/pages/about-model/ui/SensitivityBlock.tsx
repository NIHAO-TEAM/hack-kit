import { Alert, Stack, Table, Text, Title } from '@mantine/core';
import { useQuery } from '@tanstack/react-query';
import { featureTitle } from '@entities/model-config';
import { ApiError } from '@shared/api';
import { formatMoney } from '@shared/lib/format';
import { StateError, StateLoading } from '@shared/ui';
import { sensitivityOptions, type ModelSensitivity } from '../api/sensitivity';

type Shift = ModelSensitivity['features'][number]['down'];

function ShiftCell({ shift }: { shift: Shift }) {
  return (
    <Table.Td ta="right" style={{ fontVariantNumeric: 'tabular-nums' }}>
      <Text span>{shift.levelChanges}</Text>
      <Text span size="xs" c="dimmed" display="block">
        {shift.topThreeChanged ? 'первая тройка изменилась' : 'первая тройка не изменилась'}
      </Text>
      <Text span size="xs" c="dimmed" display="block">
        потеря {shift.lossDeltaCents > 0 ? '+' : ''}
        {formatMoney(shift.lossDeltaCents)}
      </Text>
    </Table.Td>
  );
}

/** Блок «Насколько устойчива очередь»: сдвиг веса каждого признака на −20 % и +20 %. Только аналитику и жюри. */
export function SensitivityBlock() {
  const query = useQuery(sensitivityOptions);
  const data = query.data;

  return (
    <Stack gap="xs">
      <Title order={2} size="h3">
        Насколько устойчива очередь
      </Title>
      <Text>
        Сдвигаем вес каждого признака на 20 % вниз и вверх и смотрим, сколько клиентов меняют
        уровень и остаётся ли верх очереди прежним.
      </Text>
      {query.isPending ? <StateLoading label="Считаем чувствительность к весам" /> : null}
      {query.isError ? (
        <StateError
          title="Не удалось посчитать устойчивость"
          requestId={query.error instanceof ApiError ? query.error.requestId : null}
          onRetry={() => void query.refetch()}
        >
          Остальное описание модели от этого не зависит.
        </StateError>
      ) : null}
      {data ? (
        <>
          {data.sampled ? (
            <Alert variant="light" color="gray">
              Клиентов много, расчёт идёт на выборке из {data.customers}. Числа относятся к выборке,
              а не ко всем клиентам.
            </Alert>
          ) : null}
          <Table.ScrollContainer
            minWidth={560}
            scrollAreaProps={{
              viewportProps: {
                tabIndex: 0,
                role: 'region',
                'aria-label': 'Устойчивость очереди к весам',
              },
            }}
          >
            <Table>
              <Table.Thead>
                <Table.Tr>
                  <Table.Th>Признак</Table.Th>
                  <Table.Th ta="right">Вес −{data.shiftPercent} %</Table.Th>
                  <Table.Th ta="right">Вес +{data.shiftPercent} %</Table.Th>
                </Table.Tr>
              </Table.Thead>
              <Table.Tbody>
                {data.features.map((f) => (
                  <Table.Tr key={f.id}>
                    <Table.Td>{featureTitle(f.id)}</Table.Td>
                    <ShiftCell shift={f.down} />
                    <ShiftCell shift={f.up} />
                  </Table.Tr>
                ))}
              </Table.Tbody>
            </Table>
          </Table.ScrollContainer>
          <Text size="sm" c="dimmed">
            В ячейке — сколько клиентов сменили уровень. Это проверка на чувствительность, а не
            калибровка.
          </Text>
        </>
      ) : null}
    </Stack>
  );
}
