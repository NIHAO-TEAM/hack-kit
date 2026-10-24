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
    await waitFor(() => expect(body).toEqual({ events: 2500, ratePerSecond: 100, seed: 1 }));
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
});
