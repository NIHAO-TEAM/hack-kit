import { QueryClient } from '@tanstack/react-query';
import { HttpResponse, http } from 'msw';
import { api, getCsrfToken, setCsrfToken, toApiError } from '@shared/api';
import { server } from '../../../mocks/server';
import { meOptions } from './session';

const user = { id: 'u1', login: 'jury', role: 'jury' };

afterEach(() => setCsrfToken(null));

describe('токен CSRF при восстановленной сессии', () => {
  it('me кладёт токен из заголовка, и запись уходит с X-CSRF-Token', async () => {
    let seen: string | null = null;
    server.use(
      http.get('*/api/v1/auth/me', () =>
        HttpResponse.json(user, { headers: { 'X-CSRF-Token': 'tok-me' } }),
      ),
      http.patch('*/api/v1/model/settings', ({ request }) => {
        seen = request.headers.get('X-CSRF-Token');
        return HttpResponse.json({});
      }),
    );
    await new QueryClient().fetchQuery(meOptions);
    await api.PATCH('/api/v1/model/settings', { body: {} });
    expect(seen).toBe('tok-me');
  });

  it('пустой заголовок токен не стирает', async () => {
    setCsrfToken('tok-old');
    server.use(
      http.get('*/api/v1/auth/me', () =>
        HttpResponse.json(user, { headers: { 'X-CSRF-Token': '' } }),
      ),
    );
    await new QueryClient().fetchQuery(meOptions);
    expect(getCsrfToken()).toBe('tok-old');
  });

  it('401 токен не стирает', async () => {
    setCsrfToken('tok-old');
    server.use(http.get('*/api/v1/auth/me', () => new HttpResponse(null, { status: 401 })));
    expect(await new QueryClient().fetchQuery(meOptions)).toBeNull();
    expect(getCsrfToken()).toBe('tok-old');
  });
});

describe('сообщение 403', () => {
  it('про CSRF — «Сессия устарела, обновите страницу»', () => {
    const body = { title: 'Forbidden', detail: 'нужен заголовок X-CSRF-Token', status: 403 };
    expect(toApiError(403, body).message).toBe('Сессия устарела, обновите страницу');
  });

  it('прочие 403 без изменения текста', () => {
    const body = { title: 'Forbidden', detail: 'недостаточно прав', status: 403 };
    expect(toApiError(403, body).message).toBe('Forbidden');
  });
});
