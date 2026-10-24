import type { QueryClient } from '@tanstack/react-query';
import type { RealtimeState } from '@shared/lib';

export const EVENTS_URL = '/api/v1/events';
export const RECONNECT_MS = 3000;

/** Корневые ключи запросов, которые перезагружаются после пересчёта: экраны начинают свои ключи с них. */
export const INVALIDATED_ROOTS = [
  'customers',
  'customer',
  'summary',
  'model-settings',
  'source',
] as const;

/** Корни ключей, которые перечитываются при изменении задачи на любом экране (SSE `tasks`). */
export const TASK_ROOTS = ['customers', 'customer', 'summary'] as const;

type EventSourceLike = Pick<EventSource, 'close' | 'readyState' | 'addEventListener'> & {
  onopen: ((e: Event) => void) | null;
  onerror: ((e: Event) => void) | null;
};

export type EventSourceFactory = (url: string) => EventSourceLike;

const defaultFactory: EventSourceFactory = (url) => new EventSource(url, { withCredentials: true });

/**
 * Подписывается на SSE: на recalculated сбрасывает запросы экранов, при обрыве сообщает offline
 * и переподключается, после возврата связи перезапрашивает данные один раз. Возвращает отписку.
 */
export function connectEvents(
  queryClient: QueryClient,
  onState: (state: RealtimeState) => void,
  factory: EventSourceFactory = defaultFactory,
): () => void {
  let source: EventSourceLike | null = null;
  let retry: ReturnType<typeof setTimeout> | undefined;
  let stopped = false;
  let version: number | null = null;
  let lost = false;

  const invalidate = () => {
    for (const root of INVALIDATED_ROOTS) {
      void queryClient.invalidateQueries({ queryKey: [root] });
    }
  };

  const invalidateTasks = () => {
    for (const root of TASK_ROOTS) {
      void queryClient.invalidateQueries({ queryKey: [root] });
    }
  };

  function open() {
    const es = factory(EVENTS_URL);
    source = es;
    es.onopen = () => {
      if (lost) {
        lost = false;
        invalidate();
      }
      onState({ status: 'online', version });
    };
    es.addEventListener('recalculated', (e) => {
      try {
        const data = JSON.parse((e as MessageEvent<string>).data) as { version?: number };
        if (typeof data.version === 'number') version = data.version;
      } catch {
        // Номер версии не разобрали — всё равно сбрасываем данные.
      }
      invalidate();
      onState({ status: 'online', version });
    });
    es.addEventListener('tasks', invalidateTasks);
    es.onerror = () => {
      lost = true;
      onState({ status: 'offline', version });
      // Браузер сам переподключает EventSource, пока тот не закрыт; закрытый поднимаем вручную.
      if (es.readyState === 2 && !stopped) {
        es.close();
        retry = setTimeout(open, RECONNECT_MS);
      }
    };
  }

  open();
  return () => {
    stopped = true;
    clearTimeout(retry);
    source?.close();
  };
}
