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
});
