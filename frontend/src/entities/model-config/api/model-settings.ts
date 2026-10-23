import { queryOptions } from '@tanstack/react-query';
import { api, toApiError, type components } from '@shared/api';

export type ModelSettings = components['schemas']['ModelSettings'];
export type ModelSettingsPatch = components['schemas']['ModelSettingsPatch'];
export type DryRunResult = components['schemas']['DryRunResult'];

export const modelSettingsKey = ['model-settings'] as const;

export const settingsQuery = queryOptions({
  queryKey: modelSettingsKey,
  queryFn: async () => {
    const { data, error, response } = await api.GET('/api/v1/model/settings').catch(() => {
      throw toApiError(0, null);
    });
    if (error || !data) throw toApiError(response.status, error, response);
    return data;
  },
  retry: false,
});

async function patch(body: ModelSettingsPatch, dryRun: boolean) {
  const { data, error, response } = await api
    .PATCH('/api/v1/model/settings', { params: { query: { dryRun } }, body })
    .catch(() => {
      throw toApiError(0, null);
    });
  if (error || !data) throw toApiError(response.status, error, response);
  return data;
}

/** Предпросмотр: только считает последствия, ничего не сохраняет. */
export async function previewSettings(body: ModelSettingsPatch): Promise<DryRunResult> {
  return (await patch(body, true)) as DryRunResult;
}

/** Сохраняет новую версию настроек. */
export async function saveSettings(body: ModelSettingsPatch): Promise<ModelSettings> {
  return (await patch(body, false)) as ModelSettings;
}

/** Предпросмотр как запрос: ключ — тело, поэтому одинаковые правки не пересчитываются. */
export const previewQuery = (body: ModelSettingsPatch) =>
  queryOptions({
    queryKey: [...modelSettingsKey, 'preview', body],
    queryFn: () => previewSettings(body),
    retry: false,
    staleTime: 0,
  });
