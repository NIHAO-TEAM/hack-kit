import { queryOptions } from '@tanstack/react-query';
import { api, toApiError, type components } from '@shared/api';
import type { RetentionTask } from './task';
import { TASK_ROOT } from './task';

export type TaskStatus = components['schemas']['TaskStatus'];

/** Вид списка задач: фильтры экрана «Задачи». */
export type TaskFilter = 'mine' | 'open' | 'overdue' | 'closed';

/** Сколько задач берём за один запрос (максимум контракта). */
export const TASKS_LIMIT = 200;

export const isOpenTask = (t: Pick<RetentionTask, 'status'>) => t.status !== 'closed';

/**
 * Список задач. «Открытые» в контракте фильтром не выражены: берём страницу и отсеиваем
 * закрытые на фронте, не пересчитывая ничего, кроме выбора строк.
 */
export const tasksQuery = (filter: TaskFilter) =>
  queryOptions({
    queryKey: [TASK_ROOT, 'list', filter] as const,
    queryFn: async () => {
      const { data, error, response } = await api
        .GET('/api/v1/tasks', {
          params: {
            query: {
              assignee: filter === 'mine' ? 'me' : undefined,
              overdue: filter === 'overdue' ? true : undefined,
              status: filter === 'closed' ? 'closed' : undefined,
              limit: TASKS_LIMIT,
            },
          },
        })
        .catch(() => {
          throw toApiError(0, null);
        });
      if (error || !data) throw toApiError(response.status, error, response);
      const items = filter === 'closed' ? data.items : data.items.filter(isOpenTask);
      return { items, total: data.total };
    },
    retry: false,
  });
