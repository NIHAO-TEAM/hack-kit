import createClient, { type Middleware } from 'openapi-fetch';
import { getCsrfToken } from './csrf';
import type { paths } from './schema.gen';

const SAFE_METHODS = new Set(['GET', 'HEAD', 'OPTIONS']);

const csrf: Middleware = {
  onRequest({ request }) {
    const token = getCsrfToken();
    if (token && !SAFE_METHODS.has(request.method)) request.headers.set('X-CSRF-Token', token);
    return request;
  },
};

// Сервер под перегрузкой отвечает 503 с Retry-After: чтение повторяем один раз после паузы
// (не дольше 2 с), запись не повторяем — её повтор решает человек.
const MAX_RETRY_WAIT_MS = 2000;

async function fetchWithRetry(request: Request): Promise<Response> {
  const retry = SAFE_METHODS.has(request.method) ? request.clone() : null;
  const response = await globalThis.fetch(request);
  if (response.status !== 503 || !retry) return response;
  const seconds = Number(response.headers.get('Retry-After'));
  const wait =
    Number.isFinite(seconds) && seconds > 0 ? Math.min(seconds * 1000, MAX_RETRY_WAIT_MS) : 1000;
  await new Promise((resolve) => setTimeout(resolve, wait));
  return globalThis.fetch(retry);
}

export function createApiClient(baseUrl = '') {
  const client = createClient<paths>({
    baseUrl,
    credentials: 'include',
    // fetch берётся при каждом запросе, а не при создании клиента: так его подменяют моки и тесты
    fetch: (request) => fetchWithRetry(request),
  });
  client.use(csrf);
  return client;
}

export const api = createApiClient(globalThis.location?.origin ?? '');
