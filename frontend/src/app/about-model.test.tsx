import { screen } from '@testing-library/react';
import { HttpResponse, http } from 'msw';
import { server } from '../mocks/server';
import { renderApp } from './render-app';

function signedInAs(role: 'marketer' | 'analyst' | 'jury') {
  server.use(http.get('/api/v1/auth/me', () => HttpResponse.json({ id: 1, login: role, role })));
}

describe('экран «О модели»', () => {
  it('показывает признаки с весами, формулу и пример клиента 9', async () => {
    signedInAs('marketer');
    server.use(
      http.get('/api/v1/model/settings', () =>
        HttpResponse.json({
          weights: {},
          thresholds: { high: 70, medium: 50 },
          horizonDays: 30,
          priceBasis: 'per_period',
          asOfMode: 'snapshot',
          version: 1,
        }),
      ),
    );
    renderApp('/about');
    expect(await screen.findByRole('heading', { name: 'О модели' })).toBeInTheDocument();
    expect(screen.getAllByText('Автопродление выключено').length).toBeGreaterThan(0);
    expect(screen.getAllByText('0,80').length).toBeGreaterThan(0);
    expect(screen.getByText(/ожидаемая потеря = Σ round/)).toBeInTheDocument();
    expect(screen.getByText(/3 189,10 €/)).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Ограничения' })).toBeInTheDocument();
    expect(screen.getByText(/Калибровки на истории нет/)).toBeInTheDocument();
    expect(screen.getAllByText('Бонусы заблокированы').length).toBeGreaterThan(0);
  });

  it('блок формулы доступен с клавиатуры и подписан', async () => {
    signedInAs('marketer');
    renderApp('/about');
    const formula = await screen.findByLabelText('Формула модели');
    expect(formula).toHaveAttribute('tabindex', '0');
  });

  it('у строчного кода нет собственной прокрутки', async () => {
    signedInAs('marketer');
    renderApp('/about');
    const inline = await screen.findByText('tools/oracle');
    expect(inline).toHaveStyle({ overflow: 'visible' });
  });

  it('берёт веса из настроек сервера', async () => {
    signedInAs('analyst');
    server.use(
      http.get('/api/v1/model/settings', () =>
        HttpResponse.json({
          weights: { pwait: 0.95 },
          thresholds: { high: 75, medium: 55 },
          horizonDays: 30,
          priceBasis: 'per_period',
          asOfMode: 'snapshot',
          version: 2,
        }),
      ),
    );
    renderApp('/about');
    expect(await screen.findByText('0,95')).toBeInTheDocument();
    expect(screen.getByText(/Высокий — скор от 75/)).toBeInTheDocument();
  });

  it('при ошибке сервера предлагает повторить', async () => {
    signedInAs('analyst');
    server.use(
      http.get('/api/v1/model/settings', () =>
        HttpResponse.json({ title: 'Сбой' }, { status: 500 }),
      ),
    );
    renderApp('/about');
    expect(
      await screen.findByRole('heading', { name: 'Не удалось загрузить настройки модели' }),
    ).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Повторить' })).toBeInTheDocument();
  });

  it('аналитику показывает, насколько устойчива очередь', async () => {
    signedInAs('analyst');
    renderApp('/about');
    expect(
      await screen.findByRole('heading', { name: 'Насколько устойчива очередь' }),
    ).toBeInTheDocument();
    expect(await screen.findByText('Вес −20 %')).toBeInTheDocument();
    expect(screen.getByText('Вес +20 %')).toBeInTheDocument();
    expect(screen.getAllByText('первая тройка не изменилась').length).toBeGreaterThan(0);
    expect(screen.getByText(/а не калибровка/)).toBeInTheDocument();
    expect(screen.getByLabelText('Устойчивость очереди к весам')).toHaveAttribute('tabindex', '0');
  });

  it('при расчёте на выборке объясняет, что числа относятся к выборке', async () => {
    signedInAs('jury');
    server.use(
      http.get('/api/v1/model/sensitivity', () =>
        HttpResponse.json({
          shiftPercent: 20,
          sampled: true,
          customers: 5000,
          snapshotVersion: 3,
          settingsVersion: 1,
          features: [],
        }),
      ),
    );
    renderApp('/about');
    expect(await screen.findByText(/расчёт идёт на выборке из 5000/)).toBeInTheDocument();
  });

  it('пока считается — состояние загрузки, при ошибке — повтор без потери остального экрана', async () => {
    signedInAs('analyst');
    server.use(
      http.get('/api/v1/model/sensitivity', () =>
        HttpResponse.json({ title: 'Долго' }, { status: 503 }),
      ),
    );
    renderApp('/about');
    expect(
      await screen.findByRole('heading', { name: 'Не удалось посчитать устойчивость' }),
    ).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Ограничения' })).toBeInTheDocument();
  });
});
