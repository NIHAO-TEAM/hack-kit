import { screen } from '@testing-library/react';
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
        lossByFeature: [
          { featureId: 'susp', lossCents: 184419, customers: 2, sharePercent: 51.24 },
          { featureId: 'funds', lossCents: 134738, customers: 1, sharePercent: 37.44 },
        ],
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

  it('блока «Работа с клиентами» нет, даже если сервер прислал retention', async () => {
    server.use(marketer(), summaryWith(retention));
    renderApp('/summary');
    expect(await screen.findByText('Ожидаемая потеря за 30 дней')).toBeInTheDocument();
    expect(screen.queryByText('Работа с клиентами')).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: /Просрочено/ })).not.toBeInTheDocument();
  });

  it('без retention в ответе остальная сводка на месте', async () => {
    server.use(marketer(), summaryWith(null));
    renderApp('/summary');
    expect(await screen.findByText('Ожидаемая потеря за 30 дней')).toBeInTheDocument();
    expect(screen.queryByText('Работа с клиентами')).not.toBeInTheDocument();
  });

  it('блок «Где теряем деньги»: причины по убыванию, сумма и доля, оговорка про причинность', async () => {
    server.use(marketer(), summaryWith(null));
    renderApp('/summary');
    const block = (await screen.findByText('Где теряем деньги')).closest(
      'section, div.mantine-Card-root',
    );
    expect(block).not.toBeNull();
    const items = await screen.findAllByRole('listitem');
    const reasons = items.filter((li) => li.textContent?.includes('%'));
    expect(reasons[0]).toHaveTextContent('Услуга приостановлена');
    expect(reasons[0]).toHaveTextContent('51,24');
    expect(reasons[1]).toHaveTextContent('Не хватает средств на продление');
    expect(screen.getByText(/Это не доказанная причинность/)).toBeInTheDocument();
  });
});
