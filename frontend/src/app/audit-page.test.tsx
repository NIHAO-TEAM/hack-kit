import { screen, within } from '@testing-library/react';
import { HttpResponse, http } from 'msw';
import { renderApp } from './render-app';
import { server } from '../mocks/server';

function signedInAs(role: 'analyst' | 'marketer' | 'admin' | 'jury') {
  server.use(http.get('/api/v1/auth/me', () => HttpResponse.json({ id: 2, login: role, role })));
}

describe('журнал — отдельная страница', () => {
  it.each(['analyst', 'admin', 'jury'] as const)('%s открывает /audit и видит записи', async (role) => {
    signedInAs(role);
    renderApp('/audit');
    expect(await screen.findByRole('heading', { level: 1, name: 'Журнал' })).toBeInTheDocument();
    const log = await screen.findByRole('region', { name: 'Журнал действий' });
    expect(await within(log).findByText('Вход в систему')).toBeInTheDocument();
  });

  it.each(['analyst', 'admin', 'jury'] as const)('%s видит пункт меню «Журнал»', async (role) => {
    signedInAs(role);
    renderApp('/summary');
    const nav = await screen.findByRole('navigation', { name: 'Разделы' });
    expect(within(nav).getByRole('link', { name: 'Журнал' })).toHaveAttribute('href', '/audit');
  });

  it('маркетолог: пункта нет, прямой переход — «Недостаточно прав»', async () => {
    signedInAs('marketer');
    renderApp('/summary');
    const nav = await screen.findByRole('navigation', { name: 'Разделы' });
    expect(within(nav).queryByRole('link', { name: 'Журнал' })).not.toBeInTheDocument();
  });

  it('маркетолог на /audit не видит записей', async () => {
    signedInAs('marketer');
    renderApp('/audit');
    expect(await screen.findByText(/Недостаточно прав/i)).toBeInTheDocument();
    expect(screen.queryByRole('region', { name: 'Журнал действий' })).not.toBeInTheDocument();
  });
});
