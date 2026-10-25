import {
  Button,
  Card,
  Group,
  NumberInput,
  SegmentedControl,
  Slider,
  Stack,
  Text,
} from '@mantine/core';
import { useForm } from '@mantine/form';
import { notifications } from '@mantine/notifications';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import {
  defaultSettings,
  featureHint,
  featureTitle,
  type ModelSettings,
  type ModelSettingsPatch,
  modelSettingsKey,
  saveSettings,
  settingsQuery,
} from '@entities/model-config';
import { auditKey } from '@widgets/audit-log';
import { ApiError } from '@shared/api';
import { PreviewPanel } from './PreviewPanel';

export type FormValues = {
  weights: Record<string, number>;
  high: number;
  medium: number;
  horizonDays: number;
  priceBasis: ModelSettings['priceBasis'];
};

export function toFormValues(s: ModelSettings): FormValues {
  return {
    weights: { ...s.weights },
    high: s.thresholds.high,
    medium: s.thresholds.medium,
    horizonDays: s.horizonDays,
    priceBasis: s.priceBasis,
  };
}

export function toPatch(v: FormValues): ModelSettingsPatch {
  return {
    weights: v.weights,
    thresholds: { high: v.high, medium: v.medium },
    horizonDays: v.horizonDays,
    priceBasis: v.priceBasis,
  };
}

/** Поле ошибки сервера → поле формы: `thresholds.high` живёт в форме как `high`. */
function formField(field: string): string {
  return field.replace(/^thresholds\./, '');
}

export function SettingsForm({
  settings,
  readOnly = false,
}: {
  settings: ModelSettings;
  /** Только просмотр (гость): поля заблокированы, кнопок и расчёта эффекта нет. */
  readOnly?: boolean;
}) {
  const form = useForm<FormValues>({ initialValues: toFormValues(settings) });
  const queryClient = useQueryClient();

  const save = useMutation({
    mutationFn: () => saveSettings(toPatch(form.values)),
    onSuccess: (saved) => {
      // Новая версия: форма пересоздаётся по `key` в ModelPage. Очередь обновит живой сигнал.
      queryClient.setQueryData(settingsQuery.queryKey, saved);
      void queryClient.invalidateQueries({ queryKey: [...modelSettingsKey, 'preview'] });
      void queryClient.invalidateQueries({ queryKey: auditKey });
      notifications.show({ message: `Сохранено, версия ${saved.version}`, color: 'teal' });
    },
    onError: (e) => {
      if (e instanceof ApiError && e.status === 422) {
        form.setErrors(
          Object.fromEntries(Object.entries(e.fieldErrors).map(([f, m]) => [formField(f), m])),
        );
        return;
      }
      notifications.show({
        title: 'Не удалось сохранить',
        message: e instanceof ApiError ? e.message : 'Ошибка запроса',
        color: 'red',
      });
    },
  });

  function reset() {
    form.clearErrors();
    form.setValues({
      weights: Object.fromEntries(
        Object.keys(form.values.weights).map((id) => [
          id,
          defaultSettings.weights[id] ?? form.values.weights[id] ?? 0,
        ]),
      ),
      high: defaultSettings.thresholds.high,
      medium: defaultSettings.thresholds.medium,
      horizonDays: defaultSettings.horizonDays,
      priceBasis: defaultSettings.priceBasis,
    });
  }

  return (
    <Group align="flex-start" wrap="wrap">
      <Stack flex="1 1 420px">
        {Object.keys(form.values.weights).map((id) => (
          <Card key={id} withBorder>
            <Text fw={500}>{featureTitle(id)}</Text>
            <Text size="sm" c="dimmed">
              {featureHint(id)}
            </Text>
            <Slider
              min={0}
              max={1}
              step={0.05}
              thumbLabel={featureTitle(id)}
              disabled={readOnly}
              {...form.getInputProps(`weights.${id}`)}
            />
          </Card>
        ))}
        <Card withBorder>
          <Stack>
            <NumberInput
              label="Порог высокого уровня"
              readOnly={readOnly}
              {...form.getInputProps('high')}
            />
            <NumberInput
              label="Порог среднего уровня"
              readOnly={readOnly}
              {...form.getInputProps('medium')}
            />
            <NumberInput
              label="Горизонт, дней"
              min={1}
              readOnly={readOnly}
              {...form.getInputProps('horizonDays')}
            />
            <SegmentedControl
              aria-label="Основа цены"
              disabled={readOnly}
              data={[
                { value: 'per_period', label: 'За период' },
                { value: 'per_month', label: 'За месяц' },
              ]}
              {...form.getInputProps('priceBasis')}
            />
          </Stack>
        </Card>
      </Stack>
      <Stack flex="0 1 320px" pos="sticky" top={16}>
        {readOnly ? null : (
          <>
            <PreviewPanel patch={toPatch(form.values)} changed={form.isDirty()} />
            <Button
              onClick={() => save.mutate()}
              loading={save.isPending}
              disabled={!form.isDirty()}
            >
              Сохранить
            </Button>
            <Button variant="default" onClick={reset}>
              Сбросить к умолчанию
            </Button>
          </>
        )}
      </Stack>
    </Group>
  );
}
