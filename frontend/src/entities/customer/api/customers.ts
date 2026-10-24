import { queryOptions } from '@tanstack/react-query';
import { api, toApiError } from '@shared/api';
import type { components } from '@shared/api';

export type CustomerRow = components['schemas']['CustomerRow'];
export type RiskLevel = components['schemas']['RiskLevel'];

export type CustomerSort =
  | 'id'
  | 'email'
  | 'balance'
  | 'totalReplenished'
  | 'score'
  | 'expectedLoss'
  /** Потеря по причине из `reason`; серверу `sort` не передаётся, он сам так упорядочивает. */
  | 'reasonLoss';
export type CustomerOrder = 'asc' | 'desc';

export interface CustomerParams {
  sort: CustomerSort;
  order: CustomerOrder;
  level?: RiskLevel;
  q?: string;
  /** Причина риска (признак модели): только клиенты с ненулевой потерей по ней. */
  reason?: string;
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

/** `sort` для сервера: «по потере по причине» он применяет сам, пока задан `reason`. */
const serverSort = (params: CustomerParams) =>
  params.sort !== 'reasonLoss' ? params.sort : params.reason ? undefined : 'expectedLoss';

export const customersOffset = (page: number | undefined, limit: number) =>
  (Math.max(page ?? 1, 1) - 1) * limit;

/**
 * Ссылка на выгрузку очереди в CSV: те же поиск, уровень и сортировка, что на экране, все строки
 * без страниц. Строки «к биллингу» остаются в файле, у них своя колонка.
 */
export const customersExportHref = (params: CustomerParams) => {
  const query = new URLSearchParams();
  const sort = serverSort(params);
  if (sort) query.set('sort', sort);
  query.set('order', params.order);
  if (params.reason) query.set('reason', params.reason);
  if (params.level) query.set('level', params.level);
  if (params.q) query.set('q', params.q);
  return `/api/v1/customers/export.csv?${query}`;
};

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
              sort: serverSort(params),
              order: params.order,
              level: params.level,
              reason: params.reason,
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
