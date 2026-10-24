import { queryOptions } from '@tanstack/react-query';
import { api, toApiError, type components } from '@shared/api';

export type Summary = components['schemas']['Summary'];
export type ChurnPoint = components['schemas']['ChurnPoint'];

export const churnChartKey = ['analytics', 'churn-chart'] as const;

export const churnChartQuery = queryOptions({
  queryKey: churnChartKey,
  queryFn: async () => {
    const { data, error, response } = await api.GET('/api/v1/analytics/churn-chart').catch(() => {
      throw toApiError(0, null);
    });
    if (error || !data) throw toApiError(response.status, error, response);
    return data.points;
  },
  retry: false,
});
