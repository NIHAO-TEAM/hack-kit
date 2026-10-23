import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { HttpResponse, http } from 'msw';
import { renderApp } from './render-app';
import { server } from '../mocks/server';

function signedIn() {
  server.use(
    http.get('/api/v1/auth/me', () =>
      HttpResponse.json({ id: 1, login: 'marketer', role: 'marketer' }),
    ),
  );
}

describe('очередь удержания', () => {
  it('клик по заголовку меняет sort и order в адресе, повторный — направление', async () => {
    signedIn();
    const user = userEvent.setup();
    const { router } = renderApp('/queue');
    await screen.findByRole('table', { name: 'Клиенты под риском оттока' });

    await user.click(screen.getByRole('button', { name: 'Email' }));
    await waitFor(() =>
      expect(router.state.location.search).toMatchObject({ sort: 'email', order: 'asc' }),
    );

    await user.click(screen.getByRole('button', { name: 'Email' }));
    await waitFor(() =>
      expect(router.state.location.search).toMatchObject({ sort: 'email', order: 'desc' }),
    );
  });

  it('по умолчанию сортировка по ожидаемой потере, «Потеря» помечена убыванием', async () => {
    signedIn();
    renderApp('/queue');
    const table = await screen.findByRole('table', { name: 'Клиенты под риском оттока' });
    const header = within(table).getByRole('columnheader', { name: /Потеря/ });
    expect(header).toHaveAttribute('aria-sort', 'descending');
  });

  it('три числа с подписями и полоса «удалит биллинг»', async () => {
    signedIn();
    renderApp('/queue');
    expect(await screen.findByText('Ожидаемая потеря за 30 дней')).toBeInTheDocument();
    expect(screen.getByText(/Платежи на 30 дней × вероятность срыва/)).toBeInTheDocument();
    expect(
      await screen.findByRole('heading', { name: /Удалит биллинг в ближайшие 5 дней/ }),
    ).toBeInTheDocument();
  });

  it('пусто: «Сегодня некого спасать»', async () => {
    signedIn();
    server.use(http.get('/api/v1/customers', () => HttpResponse.json({ items: [], total: 0 })));
    renderApp('/queue');
    expect(
      await screen.findByRole('heading', { name: 'Сегодня некого спасать' }),
    ).toBeInTheDocument();
  });

  it('ошибка сервера: экран ошибки с повтором', async () => {
    signedIn();
    server.use(
      http.get('/api/v1/customers', () =>
        HttpResponse.json(
          { type: 'about:blank', title: 'Сбой', status: 500 },
          { status: 500, headers: { 'Content-Type': 'application/problem+json' } },
        ),
      ),
    );
    renderApp('/queue');
    expect(
      await screen.findByRole('heading', { name: 'Не удалось загрузить очередь' }),
    ).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Повторить' })).toBeInTheDocument();
  });

  it('нет связи: плашка с повтором', async () => {
    signedIn();
    server.use(http.get('/api/v1/customers', () => HttpResponse.error()));
    renderApp('/queue');
    expect(await screen.findByText(/Нет связи с сервером\./)).toBeInTheDocument();
  });

  it('нет прав на сервере (403): «Недостаточно прав»', async () => {
    signedIn();
    server.use(
      http.get('/api/v1/customers', () =>
        HttpResponse.json(
          { type: 'about:blank', title: 'Запрещено', status: 403 },
          { status: 403, headers: { 'Content-Type': 'application/problem+json' } },
        ),
      ),
    );
    renderApp('/queue');
    expect(await screen.findByRole('heading', { name: 'Недостаточно прав' })).toBeInTheDocument();
  });

  it('«К биллингу» стоит вместо действия', async () => {
    signedIn();
    renderApp('/queue');
    expect(await screen.findByText('к биллингу')).toBeInTheDocument();
  });
});
