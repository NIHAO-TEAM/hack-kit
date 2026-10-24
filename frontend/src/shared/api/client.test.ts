import { HttpResponse, http } from 'msw';
import { server } from '../../mocks/server';
import { createApiClient } from './client';
import { setCsrfToken } from './csrf';
import { toApiError } from './problem';

const seen: Record<string, string | null> = {};
const base = 'http://api.test';

beforeEach(() => {
  server.use(
    http.patch(`${base}/api/v1/model/settings`, ({ request }) => {
      seen.patch = request.headers.get('X-CSRF-Token');
      return HttpResponse.json({});
    }),
    http.get(`${base}/api/v1/model/settings`, ({ request }) => {
      seen.get = request.headers.get('X-CSRF-Token');
      return HttpResponse.json({});
    }),
  );
});
afterEach(() => setCsrfToken(null));

describe('клиент API', () => {
  it('добавляет X-CSRF-Token к PATCH', async () => {
    setCsrfToken('tok-1');
    await createApiClient(base).PATCH('/api/v1/model/settings', { body: {} });
    expect(seen.patch).toBe('tok-1');
  });

  it('повторяет чтение один раз после 503 с Retry-After', async () => {
    let calls = 0;
    server.use(
      http.get(`${base}/api/v1/analytics/summary`, () => {
        calls += 1;
        return calls === 1
          ? HttpResponse.json({ status: 503 }, { status: 503, headers: { 'Retry-After': '0' } })
          : HttpResponse.json({});
      }),
    );
    const { response } = await createApiClient(base).GET('/api/v1/analytics/summary');
    expect(response.status).toBe(200);
    expect(calls).toBe(2);
  });

  it('не повторяет запись после 503', async () => {
    let calls = 0;
    server.use(
      http.patch(`${base}/api/v1/model/settings`, () => {
        calls += 1;
        return HttpResponse.json({ status: 503 }, { status: 503, headers: { 'Retry-After': '0' } });
      }),
    );
    const { response } = await createApiClient(base).PATCH('/api/v1/model/settings', { body: {} });
    expect(response.status).toBe(503);
    expect(calls).toBe(1);
  });

  it('503 показывает человеку понятную фразу', () => {
    expect(toApiError(503, { title: 'Service Unavailable', status: 503 }).message).toBe(
      'Сервис перегружен, повторите через несколько секунд',
    );
  });

  it('не добавляет X-CSRF-Token к GET', async () => {
    setCsrfToken('tok-1');
    await createApiClient(base).GET('/api/v1/model/settings');
    expect(seen.get).toBeNull();
  });
});
