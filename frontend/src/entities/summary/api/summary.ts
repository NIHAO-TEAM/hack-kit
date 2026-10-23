import { queryOptions } from '@tanstack/react-query';
import { api, toApiError } from '@shared/api';

export const summaryKey = ['summary'] as const;

/** Сводка риска: три числа над очередью. */
export const summaryOptions = queryOptions({
  queryKey: summaryKey,
  queryFn: async () => {
    const { data, error, response } = await api.GET('/api/v1/analytics/summary').catch(() => {
      throw toApiError(0, null);
    });
    if (error || !data) throw toApiError(response.status, error, response);
    return data;
  },
});
