import { queryOptions } from '@tanstack/react-query';
import { api, toApiError, type components } from '@shared/api';

export type CustomerDetail = components['schemas']['CustomerDetail'];
export type CustomerFeature = components['schemas']['Feature'];
export type CustomerAction = components['schemas']['Action'];

export const customerKey = (id: number) => ['customer', id] as const;

export const customerQuery = (id: number) =>
  queryOptions({
    queryKey: customerKey(id),
    queryFn: async () => {
      const { data, error, response } = await api
        .GET('/api/v1/customers/{id}', { params: { path: { id } } })
        .catch(() => {
          throw toApiError(0, null);
        });
      if (error || !data) throw toApiError(response.status, error, response);
      return data;
    },
    retry: false,
  });
