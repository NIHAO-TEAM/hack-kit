import { HttpResponse, http } from 'msw';

const entries = [
  {
    id: 3,
    at: '2026-10-24T09:15:00Z',
    actor: 'analyst',
    action: 'settingsChanged',
    summary: 'Изменены настройки модели',
    changes: [{ field: 'weights.noauto', from: '0.55', to: '0.6' }],
  },
  {
    id: 2,
    at: '2026-10-24T09:10:00Z',
    actor: 'analyst',
    action: 'login',
    summary: 'Вход в систему',
    changes: [],
  },
  {
    id: 1,
    at: '2026-10-24T09:05:00Z',
    actor: 'analyst',
    action: 'loginFailed',
    summary: 'Неудачный вход: неверный логин или пароль',
    changes: [],
  },
];

export const auditHandlers = [
  http.get('/api/v1/audit', ({ request }) => {
    const url = new URL(request.url);
    const limit = Number(url.searchParams.get('limit') ?? 50);
    const offset = Number(url.searchParams.get('offset') ?? 0);
    return HttpResponse.json({
      items: entries.slice(offset, offset + limit),
      total: entries.length,
    });
  }),
];
