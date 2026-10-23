import { Card, NumberInput, SegmentedControl, Slider, Stack, Text } from '@mantine/core';
import { useForm } from '@mantine/form';
import { featureHint, featureTitle, type ModelSettings } from '@entities/model-config';

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

export function SettingsForm({ settings }: { settings: ModelSettings }) {
  const form = useForm<FormValues>({ initialValues: toFormValues(settings) });

  return (
    <Stack>
      {Object.keys(form.values.weights).map((id) => (
        <Card key={id} withBorder>
          <Text fw={500} id={`weight-${id}`}>
            {featureTitle(id)}
          </Text>
          <Text size="sm" c="dimmed">
            {featureHint(id)}
          </Text>
          <Slider
            min={0}
            max={1}
            step={0.05}
            aria-labelledby={`weight-${id}`}
            {...form.getInputProps(`weights.${id}`)}
          />
        </Card>
      ))}
      <Card withBorder>
        <Stack>
          <NumberInput label="Порог высокого уровня" {...form.getInputProps('high')} />
          <NumberInput label="Порог среднего уровня" {...form.getInputProps('medium')} />
          <NumberInput label="Горизонт, дней" min={1} {...form.getInputProps('horizonDays')} />
          <SegmentedControl
            aria-label="Основа цены"
            data={[
              { value: 'per_period', label: 'За период' },
              { value: 'per_month', label: 'За месяц' },
            ]}
            {...form.getInputProps('priceBasis')}
          />
        </Stack>
      </Card>
    </Stack>
  );
}
