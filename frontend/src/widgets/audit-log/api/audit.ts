import { keepPreviousData, queryOptions } from '@tanstack/react-query';
import { api, toApiError, type components } from '@shared/api';

export type AuditEntry = components['schemas']['AuditEntry'];
export type AuditChange = components['schemas']['AuditChange'];

export const auditKey = ['audit'] as const;
export const PAGE_SIZE = 10;

export const auditQuery = (page: number) =>
  queryOptions({
    queryKey: [...auditKey, page],
    queryFn: async () => {
      const { data, error, response } = await api
        .GET('/api/v1/audit', { params: { query: { limit: PAGE_SIZE, offset: page * PAGE_SIZE } } })
        .catch(() => {
          throw toApiError(0, null);
        });
      if (error || data === undefined) throw toApiError(response.status, error, response);
      return data;
    },
    placeholderData: keepPreviousData,
    retry: false,
  });
