import { queryOptions } from '@tanstack/react-query';
import { api, toApiError, type components } from '@shared/api';

export type SourceStatus = components['schemas']['SourceStatus'];
export type ServicePatch = components['schemas']['ServicePatch'];
export type AccountPatch = components['schemas']['AccountPatch'];
export type EmulatorAction = components['schemas']['EmulatorAction'];
export type SnapshotAccepted = components['schemas']['SnapshotAccepted'];

export const sourceKey = ['source'] as const;
export const qualityKey = ['summary', 'quality'] as const;

type Result<T> = { data?: T; error?: unknown; response: Response };

function unwrap<T>({ data, error, response }: Result<T>): T {
  if (error || data === undefined) throw toApiError(response.status, error, response);
  return data;
}

const network = () => {
  throw toApiError(0, null);
};

export const statusQuery = queryOptions({
  queryKey: [...sourceKey, 'status'],
  queryFn: async () => unwrap(await api.GET('/api/v1/source/status').catch(network)),
  retry: false,
});

/** Счётчики проверок качества данных — из сводки. */
export const qualityQuery = queryOptions({
  queryKey: qualityKey,
  queryFn: async () => {
    const summary = unwrap(await api.GET('/api/v1/analytics/summary').catch(network));
    return summary.qualityCounts ?? {};
  },
  retry: false,
});

export async function patchService(id: number, body: ServicePatch) {
  return unwrap(
    await api
      .PATCH('/api/v1/source/services/{id}', { params: { path: { id } }, body })
      .catch(network),
  );
}

export async function patchAccount(id: number, body: AccountPatch) {
  return unwrap(
    await api
      .PATCH('/api/v1/source/accounts/{id}', { params: { path: { id } }, body })
      .catch(network),
  );
}

export async function emulatorTick(days: number): Promise<SnapshotAccepted> {
  return unwrap(await api.POST('/api/v1/source/emulator/tick', { body: { days } }).catch(network));
}

export async function emulatorActions(actions: EmulatorAction[]): Promise<SnapshotAccepted> {
  return unwrap(
    await api.POST('/api/v1/source/emulator/actions', { body: { actions } }).catch(network),
  );
}

/** Загрузка файла снимка. Ошибки 422 приходят по полям (`fieldErrors`). */
export async function putSnapshot(file: File): Promise<SnapshotAccepted> {
  let body: unknown;
  try {
    body = JSON.parse(await file.text());
  } catch {
    throw toApiError(400, { title: 'Файл не похож на JSON', status: 400 });
  }
  return unwrap(
    await api
      .PUT('/api/v1/source/snapshot', { body: body as components['schemas']['Snapshot'] })
      .catch(network),
  );
}

export type SourcePolling = components['schemas']['SourcePolling'];

/** Включает опрос REST-биллинга (`paused: false`) или ставит его на паузу. */
export async function setPolling(paused: boolean): Promise<SourcePolling> {
  return unwrap(await api.PUT('/api/v1/source/polling', { body: { paused } }).catch(network));
}
