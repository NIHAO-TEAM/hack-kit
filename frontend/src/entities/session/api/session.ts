import { queryOptions } from '@tanstack/react-query';
import { api, setCsrfToken, toApiError } from '@shared/api';

export const sessionKey = ['session'] as const;

/** Текущий пользователь; `null` — сессии нет (401). */
export const meOptions = queryOptions({
  queryKey: sessionKey,
  queryFn: async () => {
    const { data, error, response } = await api.GET('/api/v1/auth/me').catch(() => {
      throw toApiError(0, null);
    });
    if (response.status === 401) return null;
    if (error) throw toApiError(response.status, error, response);
    // сервер отдаёт токен CSRF сессии заголовком ответа; пустой заголовок прежний токен не стирает
    const csrfToken = response.headers.get('X-CSRF-Token');
    if (csrfToken) setCsrfToken(csrfToken);
    return data;
  },
  staleTime: 60_000,
  retry: false,
});

export async function login(credentials: { login: string; password: string }) {
  const { data, error, response } = await api
    .POST('/api/v1/auth/login', { body: credentials })
    .catch(() => {
      throw toApiError(0, null);
    });
  if (error || !data) throw toApiError(response.status, error, response);
  setCsrfToken(data.csrfToken ?? null);
  return data.user;
}

/** Гостевой вход без пароля (роль viewer). Выключен на сервере — 404. */
export async function guestLogin() {
  const { data, error, response } = await api.POST('/api/v1/auth/guest').catch(() => {
    throw toApiError(0, null);
  });
  if (error || !data) throw toApiError(response.status, error, response);
  setCsrfToken(data.csrfToken ?? null);
  return data.user;
}

/** Завершает сессию на сервере. Не вышло (не 401) — бросает ошибку, локальное состояние не трогает. */
export async function logout() {
  const { error, response } = await api.POST('/api/v1/auth/logout').catch(() => {
    throw toApiError(0, null);
  });
  if (!response.ok && response.status !== 401) throw toApiError(response.status, error, response);
  setCsrfToken(null);
}
