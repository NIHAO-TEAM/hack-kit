import { queryOptions } from '@tanstack/react-query';
import { api, toApiError } from '@shared/api';

export const modelSettingsKey = ['model', 'settings'] as const;

/** Настройки модели: веса, пороги, горизонт. */
export const modelSettingsOptions = queryOptions({
  queryKey: modelSettingsKey,
  queryFn: async () => {
    const { data, error, response } = await api.GET('/api/v1/model/settings').catch(() => {
      throw toApiError(0, null);
    });
    if (error || !data) throw toApiError(response.status, error, response);
    return data;
  },
});
