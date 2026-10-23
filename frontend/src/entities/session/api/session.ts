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

export async function logout() {
  await api.POST('/api/v1/auth/logout').catch(() => undefined);
  setCsrfToken(null);
}
