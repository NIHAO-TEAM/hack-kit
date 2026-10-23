import { Button, NumberInput, Paper, Select, Stack, TextInput, Title } from '@mantine/core';
import { useForm } from '@mantine/form';
import { useMutation } from '@tanstack/react-query';
import { patchAccount, patchService } from '../api/source';
import { eurosToCents } from '../model/quality';
import { ApiError } from '@shared/api';
import { useAfterWrite } from './useAfterWrite';

const statusOptions = [
  { value: 'active', label: 'Активна' },
  { value: 'prolong_wait', label: 'Ждёт оплаты' },
  { value: 'suspended', label: 'Приостановлена' },
  { value: 'blocked', label: 'Заблокирована' },
  { value: 'deleted', label: 'Удалена' },
];

type Status = 'active' | 'prolong_wait' | 'suspended' | 'blocked' | 'deleted';

function ServiceForm() {
  const after = useAfterWrite();
  const form = useForm({
    initialValues: {
      id: '' as number | '',
      status: null as string | null,
      autoProlong: 'keep',
      expiresAt: '',
    },
    validate: {
      id: (v) => (v === '' ? 'Укажите номер услуги' : null),
      expiresAt: (v) => (v && Number.isNaN(Date.parse(v)) ? 'Дата вида 2026-11-01T12:00' : null),
    },
  });
  const save = useMutation({
    mutationFn: () => {
      const v = form.values;
      return patchService(Number(v.id), {
        ...(v.status ? { status: v.status as Status } : {}),
        ...(v.autoProlong === 'keep' ? {} : { autoProlong: v.autoProlong === 'on' }),
        ...(v.expiresAt ? { expiresAt: new Date(`${v.expiresAt}Z`).toISOString() } : {}),
      });
    },
    onSuccess: (s) => after.done(`Услуга ${s.id} изменена, очередь пересчитывается`),
    onError: (e) => {
      if (e instanceof ApiError && e.status === 404) form.setFieldError('id', 'Такой услуги нет');
      else after.fail('Не удалось изменить услугу', e);
    },
  });
  return (
    <form onSubmit={form.onSubmit(() => save.mutate())}>
      <Stack gap="sm">
        <Title order={3} size="h5">
          Услуга
        </Title>
        <NumberInput
          label="Номер услуги"
          min={1}
          allowDecimal={false}
          {...form.getInputProps('id')}
        />
        <Select
          label="Статус"
          placeholder="Не менять"
          clearable
          data={statusOptions}
          {...form.getInputProps('status')}
        />
        <Select
          label="Автопродление"
          allowDeselect={false}
          data={[
            { value: 'keep', label: 'Не менять' },
            { value: 'on', label: 'Включено' },
            { value: 'off', label: 'Выключено' },
          ]}
          {...form.getInputProps('autoProlong')}
        />
        <TextInput
          label="Оплачено до (UTC)"
          placeholder="2026-11-01T12:00"
          {...form.getInputProps('expiresAt')}
        />
        <Button type="submit" variant="default" loading={save.isPending}>
          Изменить услугу
        </Button>
      </Stack>
    </form>
  );
}

function AccountForm() {
  const after = useAfterWrite();
  const form = useForm({
    initialValues: { id: '' as number | '', balance: '', bonus: '' },
    validate: {
      id: (v) => (v === '' ? 'Укажите номер клиента' : null),
      balance: (v) => (v && eurosToCents(v) === null ? 'Сумма в евро, например 529,84' : null),
      bonus: (v) => (v && eurosToCents(v) === null ? 'Сумма в евро, например 45,52' : null),
    },
  });
  const save = useMutation({
    mutationFn: () => {
      const v = form.values;
      return patchAccount(Number(v.id), {
        ...(v.balance ? { balance: eurosToCents(v.balance)! } : {}),
        ...(v.bonus ? { bonusBalance: eurosToCents(v.bonus)! } : {}),
      });
    },
    onSuccess: (a) => after.done(`Счёт клиента ${a.id} изменён, очередь пересчитывается`),
    onError: (e) => {
      if (e instanceof ApiError && e.status === 404) form.setFieldError('id', 'Такого клиента нет');
      else after.fail('Не удалось изменить счёт', e);
    },
  });
  return (
    <form onSubmit={form.onSubmit(() => save.mutate())}>
      <Stack gap="sm">
        <Title order={3} size="h5">
          Счёт клиента
        </Title>
        <NumberInput
          label="Номер клиента"
          min={1}
          allowDecimal={false}
          {...form.getInputProps('id')}
        />
        <TextInput
          label="Новый баланс, €"
          placeholder="Не менять"
          {...form.getInputProps('balance')}
        />
        <TextInput
          label="Новый бонус, €"
          placeholder="Не менять"
          {...form.getInputProps('bonus')}
        />
        <Button type="submit" variant="default" loading={save.isPending}>
          Изменить счёт
        </Button>
      </Stack>
    </form>
  );
}

/** Правка одной записи источника. Остальные поля остаются как были; сервер проверяет всё заново. */
export function EditPanel() {
  return (
    <Paper withBorder p="lg" radius="lg" component="section" aria-label="Правка данных">
      <Stack gap="xl">
        <Title order={2} size="h4">
          Изменить данные
        </Title>
        <ServiceForm />
        <AccountForm />
      </Stack>
    </Paper>
  );
}
