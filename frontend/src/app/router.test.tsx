import { cleanup, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { HttpResponse, http } from 'msw';
import { server } from '../mocks/server';
import { renderApp } from './render-app';

function signedInAs(role: 'marketer' | 'analyst' | 'jury') {
  server.use(http.get('/api/v1/auth/me', () => HttpResponse.json({ id: 1, login: role, role })));
}

describe('охрана маршрутов', () => {
  it('без сессии любой раздел ведёт на экран входа', async () => {
    const { router } = renderApp('/queue');
    expect(await screen.findByRole('button', { name: 'Войти' })).toBeInTheDocument();
    expect(router.state.location.pathname).toBe('/login');
  });

  it('маркетолог на адресе раздела «Модель» видит «Недостаточно прав»', async () => {
    signedInAs('marketer');
    renderApp('/model');
    expect(await screen.findByRole('heading', { name: 'Недостаточно прав' })).toBeInTheDocument();
    expect(screen.getByText(/Раздел «Модель» доступен аналитику/)).toBeInTheDocument();
  });

  it('в навигации маркетолога нет «Модели» и «Источника»', async () => {
    signedInAs('marketer');
    renderApp('/queue');
    const nav = await screen.findByRole('navigation', { name: 'Разделы' });
    expect(nav).toHaveTextContent('Очередь');
    expect(nav).toHaveTextContent('Сводка');
    expect(nav).not.toHaveTextContent('Модель');
    expect(nav).not.toHaveTextContent('Источник');
  });

  it('в навигации аналитика есть «Модель» и «Источник»', async () => {
    signedInAs('analyst');
    renderApp('/model');
    const nav = await screen.findByRole('navigation', { name: 'Разделы' });
    expect(nav).toHaveTextContent('Модель');
    expect(nav).toHaveTextContent('Источник');
  });

  it('аналитик и жюри видят ссылку «Мониторинг» на /grafana/ в новой вкладке', async () => {
    for (const role of ['analyst', 'jury'] as const) {
      signedInAs(role);
      renderApp('/queue');
      const link = await screen.findByRole('link', { name: /Мониторинг/ });
      expect(link).toHaveAttribute('href', '/grafana/');
      expect(link).toHaveAttribute('target', '_blank');
      expect(link).toHaveAttribute('rel', expect.stringContaining('noopener'));
      cleanup();
    }
  });

  it('маркетологу ссылка «Мониторинг» не показывается', async () => {
    signedInAs('marketer');
    renderApp('/queue');
    await screen.findByRole('navigation', { name: 'Разделы' });
    expect(screen.queryByRole('link', { name: /Мониторинг/ })).not.toBeInTheDocument();
  });

  it('корень ведёт на стартовую страницу роли', async () => {
    signedInAs('analyst');
    const { router } = renderApp('/');
    await waitFor(() => expect(router.state.location.pathname).toBe('/model'));
  });
});

describe('вход и выход', () => {
  it('верный пароль открывает стартовую страницу маркетолога', async () => {
    const user = userEvent.setup();
    const { router } = renderApp('/login');
    await user.type(await screen.findByLabelText('Логин'), 'marketer');
    await user.type(screen.getByLabelText('Пароль'), 'demo');
    await user.click(screen.getByRole('button', { name: 'Войти' }));
    await waitFor(() => expect(router.state.location.pathname).toBe('/queue'));
  });

  it('вход жюри открывает очередь', async () => {
    const user = userEvent.setup();
    const { router } = renderApp('/login');
    await user.type(await screen.findByLabelText('Логин'), 'jury');
    await user.type(screen.getByLabelText('Пароль'), 'demo');
    await user.click(screen.getByRole('button', { name: 'Войти' }));
    await waitFor(() => expect(router.state.location.pathname).toBe('/queue'));
  });

  it('неверный пароль: текст ошибки, логин сохранён', async () => {
    const user = userEvent.setup();
    renderApp('/login');
    await user.type(await screen.findByLabelText('Логин'), 'marketer');
    await user.type(screen.getByLabelText('Пароль'), 'wrong');
    await user.click(screen.getByRole('button', { name: 'Войти' }));
    expect(await screen.findByText('Неверный логин или пароль')).toBeInTheDocument();
    expect(screen.getByLabelText('Логин')).toHaveValue('marketer');
  });

  it('429: текст говорит, через сколько минут повторить', async () => {
    const user = userEvent.setup();
    renderApp('/login');
    await user.type(await screen.findByLabelText('Логин'), 'toomany');
    await user.type(screen.getByLabelText('Пароль'), 'demo');
    await user.click(screen.getByRole('button', { name: 'Войти' }));
    expect(await screen.findByText(/через 5 мин/)).toBeInTheDocument();
  });

  it('выход возвращает на экран входа', async () => {
    const user = userEvent.setup();
    const { router } = renderApp('/login');
    await user.type(await screen.findByLabelText('Логин'), 'marketer');
    await user.type(screen.getByLabelText('Пароль'), 'demo');
    await user.click(screen.getByRole('button', { name: 'Войти' }));
    await user.click(await screen.findByRole('button', { name: 'Выйти' }));
    await waitFor(() => expect(router.state.location.pathname).toBe('/login'));
    await router.navigate({ to: '/queue' });
    await waitFor(() => expect(router.state.location.pathname).toBe('/login'));
  });

  it('сервер не завершил сессию (403): остаёмся в панели и видим сообщение', async () => {
    const user = userEvent.setup();
    server.use(
      http.post('/api/v1/auth/logout', () =>
        HttpResponse.json(
          { type: 'about:blank', title: 'Запрещено', status: 403 },
          { status: 403, headers: { 'Content-Type': 'application/problem+json' } },
        ),
      ),
    );
    const { router } = renderApp('/login');
    await user.type(await screen.findByLabelText('Логин'), 'marketer');
    await user.type(screen.getByLabelText('Пароль'), 'demo');
    await user.click(screen.getByRole('button', { name: 'Войти' }));
    await user.click(await screen.findByRole('button', { name: 'Выйти' }));
    expect(await screen.findByText('Не удалось выйти')).toBeInTheDocument();
    expect(router.state.location.pathname).toBe('/queue');
  });

  it.each([403, 500])(
    'сервер не завершил сессию (%i, пустое тело): остаёмся в панели и видим сообщение',
    async (status) => {
      const user = userEvent.setup();
      server.use(http.post('/api/v1/auth/logout', () => new HttpResponse(null, { status })));
      const { router } = renderApp('/login');
      await user.type(await screen.findByLabelText('Логин'), 'marketer');
      await user.type(screen.getByLabelText('Пароль'), 'demo');
      await user.click(screen.getByRole('button', { name: 'Войти' }));
      await user.click(await screen.findByRole('button', { name: 'Выйти' }));
      expect(await screen.findByText('Не удалось выйти')).toBeInTheDocument();
      expect(router.state.location.pathname).toBe('/queue');
    },
  );
});

describe('ошибка загрузки сессии', () => {
  it('нет связи: плашка и повтор вместо экрана ошибки библиотеки', async () => {
    server.use(http.get('/api/v1/auth/me', () => HttpResponse.error()));
    renderApp('/queue');
    expect(await screen.findByText(/Нет связи с сервером\./)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Повторить сейчас' })).toBeInTheDocument();
  });

  it('ошибка сервера 500: экран ошибки с кнопкой «Повторить»', async () => {
    server.use(
      http.get('/api/v1/auth/me', () =>
        HttpResponse.json(
          { type: 'about:blank', title: 'Сбой', status: 500 },
          { status: 500, headers: { 'Content-Type': 'application/problem+json' } },
        ),
      ),
    );
    renderApp('/queue');
    expect(
      await screen.findByRole('heading', { name: 'Не удалось загрузить страницу' }),
    ).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Повторить' })).toBeInTheDocument();
  });
});

describe('AppShell', () => {
  it('смена темы сохраняется', async () => {
    signedInAs('marketer');
    localStorage.removeItem('mantine-color-scheme-value');
    renderApp('/queue');
    await userEvent.click(await screen.findByLabelText('Светлая'));
    expect(localStorage.getItem('mantine-color-scheme-value')).toBe('light');
    expect(document.documentElement.getAttribute('data-mantine-color-scheme')).toBe('light');
  });
});
