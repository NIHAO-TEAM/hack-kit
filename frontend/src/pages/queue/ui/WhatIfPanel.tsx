import { Button, Card, Group, List, Loader, Select, Slider, Stack, Text } from '@mantine/core';
import { useDebouncedValue } from '@mantine/hooks';
import { notifications } from '@mantine/notifications';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import {
  dryRunText,
  featureHint,
  featureTitle,
  modelSettingsKey,
  previewQuery,
  saveSettings,
  settingsQuery,
  type DryRunResult,
  type ModelSettings,
  type ModelSettingsPatch,
} from '@entities/model-config';
import { can, type Role } from '@entities/session';
import { ApiError } from '@shared/api';

export const WHATIF_DELAY_MS = 300;

const levelLabel: Record<string, string> = {
  low: 'низкий',
  medium: 'средний',
  high: 'высокий',
  unscored: 'без оценки',
  lost: 'потерян',
};

/** «medium->high» → «средний → высокий» */
function transition(key: string): string {
  return key
    .split('->')
    .map((l) => levelLabel[l] ?? l)
    .join(' → ');
}

function Changes({ result }: { result: DryRunResult }) {
  const rows = Object.entries(result.levelChanges).filter(([, n]) => n > 0);
  return (
    <Stack gap="xs">
      <Text role="status">{dryRunText(result)}</Text>
      {rows.length > 0 ? (
        <List size="sm" aria-label="Кто сменит уровень">
          {rows.map(([key, n]) => (
            <List.Item key={key}>
              {transition(key)}: {n}
            </List.Item>
          ))}
        </List>
      ) : null}
    </Stack>
  );
}

function Body({ settings, role }: { settings: ModelSettings; role: Role }) {
  const editable = can(role, 'model.edit');
  const ids = Object.keys(settings.weights);
  const [feature, setFeature] = useState(ids.includes('noauto') ? 'noauto' : (ids[0] ?? ''));
  const current = settings.weights[feature] ?? 0;
  const [picked, setPicked] = useState<{ feature: string; value: number } | null>(null);
  const value = picked?.feature === feature ? picked.value : current;
  const changed = value !== current;

  const patch: ModelSettingsPatch = { weights: { [feature]: value } };
  const [debounced] = useDebouncedValue(patch, WHATIF_DELAY_MS);
  const settled = JSON.stringify(debounced) === JSON.stringify(patch);
  const preview = useQuery({ ...previewQuery(debounced), enabled: editable && changed });
  const error = preview.error instanceof ApiError ? preview.error : null;

  const queryClient = useQueryClient();
  const apply = useMutation({
    mutationFn: () => saveSettings(patch),
    onSuccess: (saved) => {
      // Таблица перестроится по живому сигналу; здесь только новая версия настроек.
      queryClient.setQueryData(settingsQuery.queryKey, saved);
      void queryClient.invalidateQueries({ queryKey: [...modelSettingsKey, 'preview'] });
      setPicked(null);
      notifications.show({ message: `Применено, версия ${saved.version}`, color: 'teal' });
    },
    onError: (e) =>
      notifications.show({
        title: 'Не удалось применить',
        message: e instanceof ApiError ? e.message : 'Ошибка запроса',
        color: 'red',
      }),
  });

  let result;
  if (!changed) result = <Text c="dimmed">Сдвиньте вес — здесь появится предпросмотр.</Text>;
  else if (!editable) result = <Text c="dimmed">Предпросмотр последствий доступен аналитику.</Text>;
  else if (!settled || preview.isFetching) result = <Loader size="sm" aria-label="Считаем" />;
  else if (error)
    result = (
      <Text c="red" role="alert">
        {error.status === 422 ? 'Проверьте значения' : 'Предпросмотр недоступен'}
      </Text>
    );
  else if (preview.data) result = <Changes result={preview.data} />;

  return (
    <Stack>
      <Select
        label="Признак"
        data={ids.map((id) => ({ value: id, label: featureTitle(id) }))}
        value={feature}
        allowDeselect={false}
        onChange={(v) => v && setFeature(v)}
      />
      <Text size="sm" c="dimmed">
        {featureHint(feature)}. Сейчас вес {current.toLocaleString('ru-RU')}.
      </Text>
      <Slider
        min={0}
        max={1}
        step={0.05}
        value={value}
        onChange={(v) => setPicked({ feature, value: v })}
        disabled={!editable}
        thumbLabel={`Вес: ${featureTitle(feature)}`}
      />
      <Card withBorder aria-label="Что изменится">
        <Stack gap="xs">
          <Text fw={500}>Что изменится</Text>
          {result}
        </Stack>
      </Card>
      {editable ? (
        <Group>
          <Button onClick={() => apply.mutate()} loading={apply.isPending} disabled={!changed}>
            Применить
          </Button>
        </Group>
      ) : (
        <Text size="sm" c="dimmed">
          Менять вес может аналитик. Вам доступен просмотр.
        </Text>
      )}
    </Stack>
  );
}

/** Панель «Что если»: ползунок веса, последствия считает сервер (`dryRun`). */
export function WhatIfPanel({ role }: { role: Role }) {
  const settings = useQuery(settingsQuery);
  return (
    <Card withBorder aria-label="Что если">
      {settings.data ? (
        <Body key={settings.data.version} settings={settings.data} role={role} />
      ) : settings.isError ? (
        <Text c="red" role="alert">
          Не удалось загрузить настройки модели
        </Text>
      ) : (
        <Loader size="sm" aria-label="Загрузка настроек" />
      )}
    </Card>
  );
}
