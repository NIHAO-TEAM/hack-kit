import { screen, waitFor } from '@testing-library/react';
import { HttpResponse, http } from 'msw';
import { renderApp } from './render-app';
import { server } from '../mocks/server';

describe('сводка', () => {
  it('три числа, уровни и график: факт сплошной, прогноз штриховой, «сейчас» подписано', async () => {
    server.use(
      http.get('/api/v1/auth/me', () =>
        HttpResponse.json({ id: 1, login: 'marketer', role: 'marketer' }),
      ),
    );
    renderApp('/summary');
    expect(await screen.findByText('Ожидаемая потеря за 30 дней')).toBeInTheDocument();
    const chart = await screen.findByRole('img', { name: /График оттока по неделям/ });
    expect(chart.querySelector('[data-testid="chart-forecast"]')).toHaveAttribute(
      'stroke-dasharray',
    );
    expect(chart.querySelector('[data-testid="chart-actual"]')).not.toHaveAttribute(
      'stroke-dasharray',
    );
    expect(chart).toHaveTextContent('сейчас');
    expect(screen.getByText(/срок услуги плюс 5 дней/)).toBeInTheDocument();
  });

  const marketer = () =>
    http.get('/api/v1/auth/me', () =>
      HttpResponse.json({ id: 1, login: 'marketer', role: 'marketer' }),
    );
  const summaryWith = (retention: unknown) =>
    http.get('/api/v1/analytics/summary', () =>
      HttpResponse.json({
        levels: { high: 3, medium: 2, low: 1, unscored: 0, lost: 4 },
        expectedLossCents: 359907,
        atRiskCents: 690046,
        lostCents: 736834,
        version: 7,
        ...(retention ? { retention } : {}),
      }),
    );
  const retention = {
    tasksInWork: 1,
    tasksContacted: 0,
    tasksPostponed: 0,
    tasksOverdue: 2,
    customersInWork: 1,
    lossInWorkCents: 318910,
    highWithoutTask: 2,
    highWithoutTaskLossCents: 30000,
    mediumWithoutTask: 0,
    mediumWithoutTaskLossCents: 0,
  };

  it('блок «Работа с клиентами»: в работе, просрочено и высокий риск без работы со ссылками', async () => {
    server.use(marketer(), summaryWith(retention));
    renderApp('/summary');
    expect(
      await screen.findByText(
        'В работе 1 клиент · 3 189,10 € из 3 599,07 € ожидаемой потери (89 %)',
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Просрочено: 2' })).toHaveAttribute(
      'href',
      '/tasks?filter=overdue',
    );
    expect(
      screen.getByRole('link', { name: 'Высокий риск без работы: 2 клиента · 300,00 €' }),
    ).toHaveAttribute('href', '/queue?task=none&level=high');
  });

  it('числа блока обновляются после события (повторный запрос сводки)', async () => {
    let current = retention;
    server.use(
      marketer(),
      http.get('/api/v1/analytics/summary', () =>
        HttpResponse.json({
          levels: { high: 3, medium: 2, low: 1, unscored: 0, lost: 4 },
          expectedLossCents: 359907,
          atRiskCents: 690046,
          lostCents: 736834,
          version: 7,
          retention: current,
        }),
      ),
    );
    const { queryClient } = renderApp('/summary');
    await screen.findByRole('link', { name: 'Просрочено: 2' });
    current = { ...retention, tasksOverdue: 5 };
    await queryClient.invalidateQueries({ queryKey: ['summary'] });
    await waitFor(() =>
      expect(screen.getByRole('link', { name: 'Просрочено: 5' })).toBeInTheDocument(),
    );
  });

  it('без блока retention в ответе блок скрыт, остальная сводка на месте', async () => {
    server.use(marketer(), summaryWith(null));
    renderApp('/summary');
    expect(await screen.findByText('Ожидаемая потеря за 30 дней')).toBeInTheDocument();
    expect(screen.queryByText('Работа с клиентами')).not.toBeInTheDocument();
  });
});
