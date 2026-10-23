import { screen, within } from '@testing-library/react';
import { HttpResponse, http } from 'msw';
import { renderApp } from './render-app';
import { server } from '../mocks/server';

function signedInAs(role: 'analyst' | 'marketer') {
  server.use(http.get('/api/v1/auth/me', () => HttpResponse.json({ id: 2, login: role, role })));
}

describe('журнал действий', () => {
  it('на экране «Модель» запись показывает кто, когда и было → стало', async () => {
    signedInAs('analyst');
    renderApp('/model');
    const log = await screen.findByRole('region', { name: 'Журнал действий' });
    expect(await within(log).findByText('Изменены настройки модели')).toBeInTheDocument();
    expect(within(log).getByText('Вес: Автопродление выключено: 0.55 → 0.6')).toBeInTheDocument();
    expect(within(log).getAllByText('analyst').length).toBeGreaterThan(0);
  });

  it('на экране «Источник» журнал тоже есть', async () => {
    signedInAs('analyst');
    renderApp('/source');
    const log = await screen.findByRole('region', { name: 'Журнал действий' });
    expect(await within(log).findByText('Вход в систему')).toBeInTheDocument();
  });

  it('пустой журнал — приглашение, не пустое место', async () => {
    signedInAs('analyst');
    server.use(http.get('/api/v1/audit', () => HttpResponse.json({ items: [], total: 0 })));
    renderApp('/source');
    expect(await screen.findByText('Записей пока нет')).toBeInTheDocument();
  });

  it('ошибка журнала не ломает экран', async () => {
    signedInAs('analyst');
    server.use(
      http.get('/api/v1/audit', () =>
        HttpResponse.json(
          { type: 'about:blank', title: 'журнал недоступен', status: 500 },
          { status: 500, headers: { 'Content-Type': 'application/problem+json' } },
        ),
      ),
    );
    renderApp('/model');
    expect(await screen.findByText('Не удалось загрузить журнал')).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Модель' })).toBeInTheDocument();
  });
});
