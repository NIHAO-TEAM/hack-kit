import { screen, within } from '@testing-library/react';
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

  it('раскрыть услугу 19 — видны funds, susp, вклад и формула', async () => {
    const user = userEvent.setup();
    renderApp('/queue/9');
    await user.click(await screen.findByRole('button', { name: /Услуга 19/ }));
    const table = await screen.findByRole('table', { name: 'Признаки риска' });
    const rows = within(table).getAllByRole('row');
    expect(rows[1]).toHaveTextContent('funds');
    expect(within(table).getByText('susp')).toBeInTheDocument();
    expect(within(table).getByText('Вклад')).toBeInTheDocument();
    expect(screen.getAllByText(/^P = 1 − \(1 − 0,7·1\)/).length).toBeGreaterThan(0);
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
});
