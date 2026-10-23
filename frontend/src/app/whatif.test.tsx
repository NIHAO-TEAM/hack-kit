import { fireEvent, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { HttpResponse, http } from 'msw';
import { renderApp } from './render-app';
import { server } from '../mocks/server';

function signedIn(role: 'analyst' | 'marketer') {
  server.use(http.get('/api/v1/auth/me', () => HttpResponse.json({ id: 1, login: role, role })));
}

describe('«что если» в очереди', () => {
  it('аналитик: вес «автопродление выключено» 0,85 → 1 клиент, +200,69 €', async () => {
    signedIn('analyst');
    const user = userEvent.setup();
    renderApp('/queue');
    await user.click(await screen.findByRole('button', { name: 'Что если' }));
    const slider = await screen.findByRole('slider', { name: /Автопродление выключено/ });
    slider.focus();
    // с нуля до 0,85: 17 шагов по 0,05
    fireEvent.keyDown(slider, { key: 'Home' });
    for (let i = 0; i < 17; i++) fireEvent.keyDown(slider, { key: 'ArrowRight' });
    const status = await screen.findByText(/сменят уровень 1 клиент, потеря \+200,69/, undefined, {
      timeout: 3000,
    });
    expect(status).toBeInTheDocument();
    expect(screen.getByText('средний → высокий: 1')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Применить' })).toBeEnabled();
  });

  it('маркетолог: «Применить» нет, только просмотр', async () => {
    signedIn('marketer');
    const user = userEvent.setup();
    renderApp('/queue');
    await user.click(await screen.findByRole('button', { name: 'Что если' }));
    expect(await screen.findByText(/Вам доступен просмотр/)).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Применить' })).not.toBeInTheDocument();
  });
});
