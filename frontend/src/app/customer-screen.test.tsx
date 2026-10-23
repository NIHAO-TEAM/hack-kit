import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { HttpResponse, http } from 'msw';
import { renderApp } from './render-app';

beforeEach(() => {
  sessionStorage.setItem('mock-session', 'marketer');
});

describe('экран «Карточка клиента»', () => {
  it('клиент 9: потеря, главное действие с ценой и пояснение суммы', async () => {
    renderApp('/queue/9');
    expect(await screen.findByRole('heading', { name: 'Клиент 9' })).toBeInTheDocument();
    expect(screen.getAllByText(/3\s459,27\s€/).length).toBeGreaterThan(0);
    expect(screen.getByText(/Пополнить на 529,84/)).toBeInTheDocument();
    expect(screen.getByText(/−1\s067,48\s€/)).toBeInTheDocument();
    expect(screen.getByText(/платежи на 30 дней × вероятность срыва/i)).toBeInTheDocument();
  });

  it('карточка клиента 9: признаки по убыванию вклада и формула с числами ответа', async () => {
    renderApp('/queue/9');
    const table = await screen.findByRole('table', { name: 'Признаки риска клиента' });
    const rows = within(table).getAllByRole('row');
    expect(rows[1]).toHaveTextContent('noauto');
    expect(rows[2]).toHaveTextContent('fail');
    expect(within(table).getByText('funds')).toBeInTheDocument();
    expect(within(table).getByText('susp')).toBeInTheDocument();
    expect(within(table).getByText('Вклад')).toBeInTheDocument();
    expect(screen.getByText(/^P = 1 − \(1 − 0,55·1\)/)).toHaveTextContent('= 0,88');
  });

  it('раскрытая услуга не выдаёт признаки клиента за свои', async () => {
    const user = userEvent.setup();
    renderApp('/queue/9');
    await screen.findByRole('table', { name: /Признаки риска/ });
    const control = document.querySelector<HTMLElement>('[data-accordion-control]');
    if (!control) throw new Error('нет раскрывающейся услуги');
    await user.click(control);
    expect(screen.getAllByRole('table', { name: /Признаки риска/ })).toHaveLength(1);
  });

  it('нечисловой адрес — «Такого клиента нет»', async () => {
    renderApp('/queue/abc');
    expect(await screen.findByText('Такого клиента нет')).toBeInTheDocument();
  });

  it('клиент 2: «потерян», сумма «уже потеряно»', async () => {
    renderApp('/queue/2');
    expect(await screen.findByText(/потерян: все услуги заблокированы/)).toBeInTheDocument();
    expect(screen.getByText('Уже потеряно')).toBeInTheDocument();
  });

  it('несуществующий клиент — «Такого клиента нет»', async () => {
    renderApp('/queue/99999');
    expect(await screen.findByText('Такого клиента нет')).toBeInTheDocument();
  });

  it('нет связи — плашка с повтором', async () => {
    const { server } = await import('../mocks/server');
    server.use(http.get('/api/v1/customers/:id', () => HttpResponse.error()));
    renderApp('/queue/9');
    expect(await screen.findByText(/Нет связи с сервером/)).toBeInTheDocument();
  });
  it('окно спасения считается от asOf сводки, а не от часов браузера', async () => {
    vi.useFakeTimers({ toFake: ['Date'], now: new Date('2030-01-01T00:00:00Z') });
    try {
      const { server } = await import('../mocks/server');
      server.use(
        http.get('/api/v1/analytics/summary', () =>
          HttpResponse.json({
            expectedLossCents: 0,
            atRiskCents: 0,
            lostCents: 0,
            asOf: '2026-10-19T15:00:00.000Z',
          }),
        ),
      );
      renderApp('/queue/9');
      await screen.findByRole('heading', { name: 'Клиент 9' });
      await waitFor(() => expect(screen.queryByText('время модели загружается')).toBeNull());
      expect(screen.queryByText(/удалено/i)).toBeNull();
    } finally {
      vi.useRealTimers();
    }
  });

  it('пока сводка не пришла — «время модели загружается» вместо полосы', async () => {
    const { server } = await import('../mocks/server');
    server.use(http.get('/api/v1/analytics/summary', () => new Promise(() => {})));
    renderApp('/queue/9');
    await screen.findByRole('heading', { name: 'Клиент 9' });
    expect(screen.getAllByText('время модели загружается').length).toBeGreaterThan(0);
  });
});
