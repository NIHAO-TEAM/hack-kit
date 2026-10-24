import { queryOptions } from '@tanstack/react-query';
import { api, toApiError, type components } from '@shared/api';

export type StreamState = components['schemas']['StreamState'];
export type StreamReport = components['schemas']['StreamReport'];
export type StreamRequest = components['schemas']['StreamRequest'];

/** Корень ключей запросов потока: по нему SSE `streamProgress` кладёт ход прогона в кэш. */
export const STREAM_ROOT = 'stream';
export const streamKey = [STREAM_ROOT] as const;

const network = (): never => {
  throw toApiError(0, null);
};

/** Пока прогон идёт, состояние перечитывается раз в секунду — на случай, если SSE нет или оборвался. */
export const streamQuery = queryOptions({
  queryKey: streamKey,
  queryFn: async (): Promise<StreamState> => {
    const { data, error, response } = await api
      .GET('/api/v1/source/emulator/stream')
      .catch(network);
    if (error || data === undefined) throw toApiError(response.status, error, response);
    return data;
  },
  retry: false,
  refetchInterval: (query) => (query.state.data?.state === 'running' ? 1000 : false),
});

/** Запускает прогон; 409, если другой ещё идёт, 400 — число событий или скорость вне границ. */
export async function startStream(body: StreamRequest): Promise<string> {
  const { data, error, response } = await api
    .POST('/api/v1/source/emulator/stream', { body })
    .catch(network);
  if (error || data === undefined) throw toApiError(response.status, error, response);
  return data.runId;
}
