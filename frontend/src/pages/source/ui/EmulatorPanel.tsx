import { Alert, Button, Group, NumberInput, Paper, Stack, Text, Title } from '@mantine/core';
import { useMutation } from '@tanstack/react-query';
import { useState } from 'react';
import { emulatorActions, emulatorTick } from '../api/source';
import { eurosToCents } from '../model/quality';
import { useAfterWrite } from './useAfterWrite';

/** Эмулятор биллинга: двигает время и делает платежи. Это имитация — настоящий биллинг не трогаем. */
export function EmulatorPanel() {
  const after = useAfterWrite();
  const [customer, setCustomer] = useState<number | ''>('');
  const [euros, setEuros] = useState('');
  const [error, setError] = useState<string | null>(null);

  const run = useMutation({
    mutationFn: (job: () => Promise<unknown>) => job(),
    onSuccess: () => after.done('Готово, данные пересчитываются'),
    onError: (e) => after.fail('Шаг эмулятора не выполнен', e),
  });

  function withCustomer(job: (id: number) => Promise<unknown>) {
    if (customer === '') return setError('Укажите номер клиента');
    setError(null);
    run.mutate(() => job(customer));
  }

  function topUp() {
    const cents = eurosToCents(euros);
    if (cents === null || cents === 0) return setError('Сумма пополнения в евро, например 529,84');
    withCustomer((id) => emulatorActions([{ actionId: 'top_up', customerId: id, cents }]));
  }

  return (
    <Paper withBorder p="lg" radius="lg" component="section" aria-label="Эмулятор биллинга">
      <Stack gap="sm">
        <Title order={2} size="h4">
          Эмулятор биллинга
        </Title>
        <Text size="sm" c="dimmed">
          Имитация биллинга: данные меняются в нашей базе, настоящий биллинг не затрагивается.
        </Text>
        <Group>
          <Button
            variant="default"
            loading={run.isPending}
            onClick={() => run.mutate(() => emulatorTick(1))}
          >
            +24 часа
          </Button>
          <Button
            variant="default"
            loading={run.isPending}
            onClick={() =>
              run.mutate(() => emulatorActions([{ actionId: 'reset', customerId: 0 }]))
            }
          >
            Сбросить
          </Button>
        </Group>
        <NumberInput
          label="Номер клиента"
          min={1}
          allowDecimal={false}
          value={customer}
          onChange={(v) => setCustomer(typeof v === 'number' ? v : '')}
        />
        <Group align="flex-end">
          <NumberInput
            label="Сумма, €"
            placeholder="529,84"
            hideControls
            decimalSeparator=","
            decimalScale={2}
            value={euros}
            onChange={(v) => setEuros(String(v).replace('.', ','))}
          />
          <Button variant="default" loading={run.isPending} onClick={topUp}>
            Пополнить
          </Button>
        </Group>
        <Button
          variant="default"
          loading={run.isPending}
          onClick={() =>
            withCustomer((id) =>
              emulatorActions([{ actionId: 'enable_autoprolong', customerId: id }]),
            )
          }
        >
          Включить автопродление
        </Button>
        {error ? (
          <Alert color="red" role="alert">
            {error}
          </Alert>
        ) : null}
      </Stack>
    </Paper>
  );
}
