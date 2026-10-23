import { HttpResponse, http } from 'msw';

const base = {
  version: 1,
  source: 'seed',
  asOf: '2026-10-19T15:00:00Z',
  loadedAt: '2026-10-19T15:00:00Z',
  ageSeconds: 60,
  checks: [
    {
      id: 'stale_status',
      ok: false,
      message: '2 услуги в ожидании оплаты дольше 5 дней',
      items: [
        { serviceId: 11, accountId: 3 },
        { serviceId: 12, accountId: 4 },
      ],
    },
    {
      id: 'blocked_future',
      ok: true,
      message: 'Нет услуг в статусе «заблокирована» со сроком в будущем',
    },
  ],
};

let state = { ...base };

/** Опрос биллинга в моке: `?polling=paused|failing|off` в адресе, по умолчанию норма. */
function pollingMock() {
  const mode = new URLSearchParams(globalThis.location?.search).get('polling');
  if (mode === 'off') return undefined;
  if (mode === 'paused') return { paused: true, intervalSeconds: 60, errorsTotal: 0 };
  if (mode === 'failing') {
    return {
      paused: false,
      intervalSeconds: 60,
      errorsTotal: 3,
      lastError: 'биллинг ответил 503',
    };
  }
  return { paused: false, intervalSeconds: 60, errorsTotal: 0 };
}

/** Сброс состояния между тестами. */
export function resetSourceMock(): void {
  state = { ...base };
}

function problem(status: number, title: string, errors?: { field: string; message: string }[]) {
  return HttpResponse.json(
    { type: 'about:blank', title, status, errors },
    { status, headers: { 'Content-Type': 'application/problem+json' } },
  );
}

const bump = (source: string) => {
  state = { ...state, version: state.version + 1, source };
  return HttpResponse.json({
    version: state.version,
    asOf: state.asOf,
    accounts: 10,
    services: 30,
    transactions: 100,
  });
};

export const sourceHandlers = [
  http.get('/api/v1/source/status', () => HttpResponse.json({ ...state, polling: pollingMock() })),
  http.put('/api/v1/source/polling', async ({ request }) => {
    const body = (await request.json()) as { paused: boolean };
    return HttpResponse.json({ paused: body.paused, intervalSeconds: 60, errorsTotal: 0 });
  }),
  http.patch('/api/v1/source/accounts/:id', async ({ params }) => {
    if (Number(params.id) > 10) return problem(404, 'счёт не найден');
    bump('api');
    return HttpResponse.json({
      id: Number(params.id),
      email: 'user@example.test',
      balance: 0,
      withdrawBalance: 0,
      totalReplenished: 0,
      bonusBalance: 0,
      bonusState: 'locked',
    });
  }),
  http.post('/api/v1/source/emulator/tick', () => bump('emulator')),
  http.post('/api/v1/source/emulator/actions', () => bump('emulator')),
  http.put('/api/v1/source/snapshot', async ({ request }) => {
    const body = (await request.json()) as { services?: { ownerId: number }[] };
    if (body.services?.some((s) => s.ownerId === 999)) {
      return problem(422, 'снимок не прошёл проверку', [
        { field: 'services[3].ownerId', message: 'владелец 999 не найден среди счетов' },
      ]);
    }
    return bump('api');
  }),
];
