import { queryOptions } from '@tanstack/react-query';
import { api, ApiError, toApiError, type components } from '@shared/api';

export type RetentionTask = components['schemas']['RetentionTask'];
export type TaskBrief = components['schemas']['TaskBrief'];
export type TaskEvent = components['schemas']['TaskEvent'];
export type TaskStatus = components['schemas']['TaskStatus'];
export type TaskOutcome = components['schemas']['TaskOutcome'];
export type TaskChannel = components['schemas']['TaskChannel'];
export type TaskCreate = components['schemas']['TaskCreate'];
export type TaskPatch = components['schemas']['TaskPatch'];
export type TaskEventCreate = components['schemas']['TaskEventCreate'];

export const taskKey = (id: number) => ['task', id] as const;
export const assigneesKey = ['task', 'assignees'] as const;

/** 409: задачу уже изменил другой сотрудник; `task` — её текущее состояние. */
export class TaskConflictError extends ApiError {
  readonly task: RetentionTask | null;

  constructor(base: ApiError, task: RetentionTask | null) {
    super(base.status, base.message, { requestId: base.requestId });
    this.name = 'TaskConflictError';
    this.task = task;
  }

  /** Логин того, кто изменил задачу. */
  get changedBy(): string | null {
    return this.task?.updatedBy ?? null;
  }
}

function fail(status: number, body: unknown, response?: Response): never {
  const base = toApiError(status, body, response);
  if (status === 409) {
    const task = (body as { task?: RetentionTask } | null)?.task ?? null;
    throw new TaskConflictError(base, task);
  }
  throw base;
}

const network = (): never => {
  throw toApiError(0, null);
};

export const taskQuery = (id: number) =>
  queryOptions({
    queryKey: taskKey(id),
    queryFn: async () => {
      const { data, error, response } = await api
        .GET('/api/v1/tasks/{taskId}', { params: { path: { taskId: id } } })
        .catch(network);
      if (error || !data) fail(response.status, error, response);
      return data;
    },
    retry: false,
  });

export const assigneesQuery = queryOptions({
  queryKey: assigneesKey,
  queryFn: async () => {
    const { data, error, response } = await api.GET('/api/v1/tasks/assignees').catch(network);
    if (error || !data) fail(response.status, error, response);
    return data.items;
  },
  staleTime: 5 * 60_000,
  retry: false,
});

export async function takeTask(customerId: number, body: TaskCreate): Promise<RetentionTask> {
  const { data, error, response } = await api
    .POST('/api/v1/customers/{id}/task', { params: { path: { id: customerId } }, body })
    .catch(network);
  if (error || !data) fail(response.status, error, response);
  return data;
}

export async function patchTask(id: number, body: TaskPatch): Promise<RetentionTask> {
  const { data, error, response } = await api
    .PATCH('/api/v1/tasks/{taskId}', { params: { path: { taskId: id } }, body })
    .catch(network);
  if (error || !data) fail(response.status, error, response);
  return data;
}

export async function addTaskEvent(id: number, body: TaskEventCreate): Promise<RetentionTask> {
  const { data, error, response } = await api
    .POST('/api/v1/tasks/{taskId}/events', { params: { path: { taskId: id } }, body })
    .catch(network);
  if (error || !data) fail(response.status, error, response);
  return data;
}
