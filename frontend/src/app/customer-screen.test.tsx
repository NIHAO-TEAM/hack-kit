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
    await user.click(await screen.findByRole('button', { name: /Услуга 19/ }));
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
});
