import { HttpResponse, http } from 'msw';
import { server } from '../../mocks/server';
import { createApiClient } from './client';
import { setCsrfToken } from './csrf';

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

  it('не добавляет X-CSRF-Token к GET', async () => {
    setCsrfToken('tok-1');
    await createApiClient(base).GET('/api/v1/model/settings');
    expect(seen.get).toBeNull();
  });
});
