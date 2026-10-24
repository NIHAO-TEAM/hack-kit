import { queryOptions } from '@tanstack/react-query';
import { api, toApiError } from '@shared/api';
import type { components } from '@shared/api';

export type CustomerRow = components['schemas']['CustomerRow'];
export type RiskLevel = components['schemas']['RiskLevel'];

export type CustomerSort =
  'id' | 'email' | 'balance' | 'totalReplenished' | 'score' | 'expectedLoss';
export type CustomerOrder = 'asc' | 'desc';

export interface CustomerParams {
  sort: CustomerSort;
  order: CustomerOrder;
  level?: RiskLevel;
  q?: string;
  /** Строки «к биллингу»: `exclude` — без них, `only` — только они; без параметра — все. */
  billing?: 'exclude' | 'only';
  /** Номер страницы с 1; без него — первая. */
  page?: number;
  /** Размер страницы; по умолчанию максимум сервера (полоса «удалит биллинг» берёт все строки). */
  limit?: number;
}

/** Размер страницы очереди. */
export const QUEUE_PAGE_SIZE = 50;
const MAX_LIMIT = 200;

export const customersOffset = (page: number | undefined, limit: number) =>
  (Math.max(page ?? 1, 1) - 1) * limit;

export const customersKey = ['customers'] as const;

/** Рейтинг клиентов: сортировка и фильтр считает сервер, фронт строки не переупорядочивает. */
export const customersOptions = (params: CustomerParams) =>
  queryOptions({
    queryKey: [...customersKey, params] as const,
    queryFn: async () => {
      const { data, error, response } = await api
        .GET('/api/v1/customers', {
          params: {
            query: {
              sort: params.sort,
              order: params.order,
              level: params.level,
              q: params.q || undefined,
              billing: params.billing,
              limit: params.limit ?? MAX_LIMIT,
              offset: customersOffset(params.page, params.limit ?? MAX_LIMIT) || undefined,
            },
          },
        })
        .catch(() => {
          throw toApiError(0, null);
        });
      if (error || !data) throw toApiError(response.status, error, response);
      return data;
    },
  });
