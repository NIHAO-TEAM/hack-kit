import { queryOptions } from '@tanstack/react-query';
import { api, toApiError, type components } from '@shared/api';

export type ModelSensitivity = components['schemas']['ModelSensitivity'];

/** Чувствительность очереди к весам: сервер кэширует расчёт по версиям снимка и настроек. */
export const sensitivityOptions = queryOptions({
  queryKey: ['model', 'sensitivity'] as const,
  queryFn: async () => {
    const { data, error, response } = await api.GET('/api/v1/model/sensitivity').catch(() => {
      throw toApiError(0, null);
    });
    if (error || !data) throw toApiError(response.status, error, response);
    return data;
  },
  retry: false,
});
