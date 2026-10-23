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

export function createApiClient(baseUrl = '') {
  const client = createClient<paths>({ baseUrl, credentials: 'include' });
  client.use(csrf);
  return client;
}

export const api = createApiClient();
