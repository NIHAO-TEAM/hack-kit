import { HttpResponse, http } from 'msw';
import { setupServer } from 'msw/node';
import { createApiClient } from './client';
import { setCsrfToken } from './csrf';

const seen: Record<string, string | null> = {};
const server = setupServer(
  http.patch('http://api.test/api/v1/model/settings', ({ request }) => {
    seen.patch = request.headers.get('X-CSRF-Token');
    return HttpResponse.json({});
  }),
  http.get('http://api.test/api/v1/model/settings', ({ request }) => {
    seen.get = request.headers.get('X-CSRF-Token');
    return HttpResponse.json({});
  }),
);

beforeAll(() => server.listen({ onUnhandledFrame: 'error' }));
afterEach(() => setCsrfToken(null));
afterAll(() => server.close());

describe('клиент API', () => {
  it('добавляет X-CSRF-Token к PATCH', async () => {
    setCsrfToken('tok-1');
    await createApiClient('http://api.test').PATCH('/api/v1/model/settings', { body: {} });
    expect(seen.patch).toBe('tok-1');
  });

  it('не добавляет X-CSRF-Token к GET', async () => {
    setCsrfToken('tok-1');
    await createApiClient('http://api.test').GET('/api/v1/model/settings');
    expect(seen.get).toBeNull();
  });
});
