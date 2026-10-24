import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { HttpResponse, http } from 'msw';
import { renderApp } from './render-app';
import { resetSourceMock } from '../mocks/source';
import { server } from '../mocks/server';

function signedInAs(role: 'analyst' | 'marketer' | 'jury') {
  server.use(http.get('/api/v1/auth/me', () => HttpResponse.json({ id: 2, login: role, role })));
}

const report = {
  events: 2500,
  applied: 2500,
  failed: 0,
  durationMs: 47750,
  eventsPerSecond: 52.4,
  applyP50Ms: 18.5,
  applyP95Ms: 23.9,
  applyMaxMs: 99.8,
  recalcs: 1900,
  queueLagP95Ms: 8.9,
  levelsBefore: { high: 3, medium: 2, low: 1, unscored: 0, lost: 4 },
  levelsAfter: { high: 2, medium: 4, low: 0, unscored: 0, lost: 4 },
  levelChanges: [{ from: 'high', to: 'medium', count: 1 }],
  lossBeforeCents: 359814,
  lossAfterCents: 988566,
  consistency: { ok: true, diffCents: 0, version: 2501 },
};

beforeEach(resetSourceMock);

describe('панель «Поток событий»', () => {
  it('по умолчанию 2 500 событий: кнопка запускает прогон с выбранными числом и скоростью', async () => {
    signedInAs('analyst');
    let body: unknown;
    server.use(
      http.post('/api/v1/source/emulator/stream', async ({ request }) => {
        body = await request.json();
        return HttpResponse.json({ runId: 'r1' }, { status: 202 });
      }),
    );
    renderApp('/source');
    const panel = await screen.findByRole('region', { name: 'Поток событий' });
    const button = within(panel).getByRole('button', { name: /Прогнать 2\s500 событий/ });
    await userEvent.click(button);
    await waitFor(() => expect(body).toEqual({ events: 2500, ratePerSecond: 0, seed: 1 }));
  });

  it('идёт прогон: полоса хода и «Применено N из M», выбор заблокирован', async () => {
    signedInAs('analyst');
    server.use(
      http.get('/api/v1/source/emulator/stream', () =>
        HttpResponse.json({ state: 'running', runId: 'r1', events: 2500, applied: 1250 }),
      ),
    );
    renderApp('/source');
    const panel = await screen.findByRole('region', { name: 'Поток событий' });
    expect(await within(panel).findByText(/Применено 1\s250 из 2\s500/)).toBeInTheDocument();
    expect(within(panel).getByRole('progressbar', { name: 'Ход потока событий' })).toHaveAttribute(
      'aria-valuenow',
      '50',
    );
  });

  it('готово: таблица отчёта и отметка согласованности', async () => {
    signedInAs('jury');
    server.use(
      http.get('/api/v1/source/emulator/stream', () =>
        HttpResponse.json({ state: 'done', runId: 'r1', events: 2500, applied: 2500, report }),
      ),
    );
    renderApp('/source');
    const panel = await screen.findByRole('region', { name: 'Поток событий' });
    expect(
      await within(panel).findByText(/Согласованность: пересчёт с нуля совпал/),
    ).toBeInTheDocument();
    const table = within(panel).getByRole('table', { name: 'Результаты потока событий' });
    expect(within(table).getByText(/2\s500 из 2\s500/)).toBeInTheDocument();
    expect(within(table).getByText(/47,8\s*с/)).toBeInTheDocument();
    expect(within(table).getByText(/3\s598,14\s€ → 9\s885,66\s€/)).toBeInTheDocument();
    expect(within(panel).getByRole('button', { name: 'Сбросить' })).toBeInTheDocument();
  });

  it('расхождение с пересчётом показано ошибкой, а не скрыто', async () => {
    signedInAs('analyst');
    server.use(
      http.get('/api/v1/source/emulator/stream', () =>
        HttpResponse.json({
          state: 'done',
          events: 2500,
          applied: 2500,
          report: { ...report, consistency: { ok: false, diffCents: 120, version: 2501 } },
        }),
      ),
    );
    renderApp('/source');
    expect(await screen.findByRole('alert')).toHaveTextContent(/расхождение 1,20\s€/);
  });

  it('ошибка прогона и отказ запуска (409) показаны человеческим текстом', async () => {
    signedInAs('analyst');
    server.use(
      http.get('/api/v1/source/emulator/stream', () =>
        HttpResponse.json({
          state: 'failed',
          events: 2500,
          applied: 40,
          error: 'пересчёт не догнал итоговый снимок за отведённое время',
        }),
      ),
      http.post('/api/v1/source/emulator/stream', () =>
        HttpResponse.json(
          { type: 'about:blank', title: 'другой прогон потока ещё идёт', status: 409 },
          { status: 409, headers: { 'Content-Type': 'application/problem+json' } },
        ),
      ),
    );
    renderApp('/source');
    const panel = await screen.findByRole('region', { name: 'Поток событий' });
    expect(await within(panel).findByText(/пересчёт не догнал/)).toBeInTheDocument();
    await userEvent.click(within(panel).getByRole('button', { name: /Прогнать/ }));
    expect(await screen.findByText('другой прогон потока ещё идёт')).toBeInTheDocument();
  });

  it('маркетолог экрана «Источник» и панели не видит', async () => {
    signedInAs('marketer');
    renderApp('/source');
    expect(await screen.findByRole('heading', { name: 'Недостаточно прав' })).toBeInTheDocument();
    expect(screen.queryByRole('region', { name: 'Поток событий' })).not.toBeInTheDocument();
  });

  it('выбор залпа: 2 500 / 10 000 / 50 000 / 100 000, скорость по умолчанию «как можно быстрее»', async () => {
    signedInAs('analyst');
    let body: unknown;
    server.use(
      http.post('/api/v1/source/emulator/stream', async ({ request }) => {
        body = await request.json();
        return HttpResponse.json({ runId: 'r1' }, { status: 202 });
      }),
    );
    renderApp('/source');
    const panel = await screen.findByRole('region', { name: 'Поток событий' });
    expect(within(panel).getByLabelText('Скорость')).toHaveValue('Как можно быстрее');
    await userEvent.click(within(panel).getByLabelText('Залп, событий'));
    await screen.findByRole('option', { name: /^100\s000$/, hidden: true });
    const options = ['2500', '10000', '50000', '100000'].map(
      (v) =>
        screen
          .queryAllByRole('option', { hidden: true })
          .filter((o) => o.textContent?.replace(/\s/g, '') === v).length,
    );
    expect(options).toEqual([1, 1, 1, 1]);
    await userEvent.click(screen.getByRole('option', { name: /^100\s000$/, hidden: true }));
    await userEvent.click(within(panel).getByRole('button', { name: /Прогнать 100\s000 событий/ }));
    await waitFor(() => expect(body).toEqual({ events: 100000, ratePerSecond: 0, seed: 1 }));
  }, 20000);

  it('идёт залп: шесть живых графиков из окна прогона', async () => {
    signedInAs('analyst');
    const window = [
      {
        t: 1,
        accepted: 900,
        applied: 880,
        failed: 0,
        queueDepth: 300,
        batchSize: 120,
        lagP95Ms: 9.5,
        recalcs: 3,
      },
      {
        t: 2,
        accepted: 950,
        applied: 940,
        failed: 0,
        queueDepth: 120,
        batchSize: 150,
        lagP95Ms: 11,
        recalcs: 4,
      },
    ];
    server.use(
      http.get('/api/v1/source/emulator/stream', () =>
        HttpResponse.json({ state: 'running', runId: 'r1', events: 10000, applied: 1820, window }),
      ),
    );
    renderApp('/source');
    const charts = await screen.findByRole('region', { name: 'Живые графики потока' });
    for (const title of [
      'События в секунду',
      'Очередь приёма',
      'Размер пачки',
      'Задержка до очереди',
      'Пересчётов в секунду',
      'Ошибки',
    ]) {
      expect(
        within(charts).getByRole('img', { name: new RegExp(`^${title} за последние 60 секунд`) }),
      ).toBeInTheDocument();
    }
    expect(
      within(charts).getByRole('img', { name: /События в секунду.*принято 950, применено 940/ }),
    ).toBeInTheDocument();
    expect(
      within(charts).getByRole('img', { name: /Очередь приёма.*ждут записи 120/ }),
    ).toBeInTheDocument();
  });

  it('готово: строка «Ошибок 0 · потерянных 0 · согласованность 0 центов»', async () => {
    signedInAs('jury');
    server.use(
      http.get('/api/v1/source/emulator/stream', () =>
        HttpResponse.json({ state: 'done', runId: 'r1', events: 2500, applied: 2500, report }),
      ),
    );
    renderApp('/source');
    expect(
      await screen.findByText('Ошибок 0 · потерянных 0 · согласованность 0 центов'),
    ).toBeInTheDocument();
  });
});
