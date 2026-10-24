import { customersKey } from '@entities/customer';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { HttpResponse, http } from 'msw';
import { renderApp } from './render-app';
import { server } from '../mocks/server';
import { seedTask } from '../mocks/tasks';

function signedIn() {
  server.use(
    http.get('/api/v1/auth/me', () =>
      HttpResponse.json({ id: 1, login: 'marketer', role: 'marketer' }),
    ),
  );
}

describe('очередь удержания', () => {
  it('клик по заголовку меняет sort и order в адресе, повторный — направление', async () => {
    signedIn();
    const user = userEvent.setup();
    const { router } = renderApp('/queue');
    await screen.findByRole('table', { name: 'Клиенты под риском оттока' });

    await user.click(screen.getByRole('button', { name: 'Email' }));
    await waitFor(() =>
      expect(router.state.location.search).toMatchObject({ sort: 'email', order: 'asc' }),
    );

    await user.click(screen.getByRole('button', { name: 'Email' }));
    await waitFor(() =>
      expect(router.state.location.search).toMatchObject({ sort: 'email', order: 'desc' }),
    );
  });

  it('по умолчанию сортировка по ожидаемой потере, «Потеря» помечена убыванием', async () => {
    signedIn();
    renderApp('/queue');
    const table = await screen.findByRole('table', { name: 'Клиенты под риском оттока' });
    const header = within(table).getByRole('columnheader', { name: /Потеря/ });
    expect(header).toHaveAttribute('aria-sort', 'descending');
  });

  it('три числа с подписями и полоса «удалит биллинг»', async () => {
    signedIn();
    renderApp('/queue');
    expect(await screen.findByText('Ожидаемая потеря за 30 дней')).toBeInTheDocument();
    expect(screen.getByText(/Платежи на 30 дней × вероятность срыва/)).toBeInTheDocument();
    expect(
      await screen.findByRole('heading', { name: /Удалит биллинг в ближайшие 5 дней/ }),
    ).toBeInTheDocument();
  });

  it('«Скачать CSV» ведёт на выгрузку с текущей сортировкой', async () => {
    signedIn();
    const user = userEvent.setup();
    renderApp('/queue');
    await screen.findByRole('table', { name: 'Клиенты под риском оттока' });
    await user.click(screen.getByRole('button', { name: 'Email' }));
    await waitFor(() => {
      const href = screen.getByRole('link', { name: 'Скачать CSV' }).getAttribute('href') ?? '';
      expect(href).toContain('/api/v1/customers/export.csv?');
      expect(href).toContain('sort=email');
      expect(href).toContain('order=asc');
    });
  });

  it('пусто: «Сегодня некого спасать»', async () => {
    signedIn();
    server.use(http.get('/api/v1/customers', () => HttpResponse.json({ items: [], total: 0 })));
    renderApp('/queue');
    expect(
      await screen.findByRole('heading', { name: 'Сегодня некого спасать' }),
    ).toBeInTheDocument();
  });

  it('ошибка сервера: экран ошибки с повтором', async () => {
    signedIn();
    server.use(
      http.get('/api/v1/customers', () =>
        HttpResponse.json(
          { type: 'about:blank', title: 'Сбой', status: 500 },
          { status: 500, headers: { 'Content-Type': 'application/problem+json' } },
        ),
      ),
    );
    renderApp('/queue');
    expect(
      await screen.findByRole('heading', { name: 'Не удалось загрузить очередь' }),
    ).toBeInTheDocument();
    expect(screen.getAllByRole('button', { name: 'Повторить' }).length).toBeGreaterThan(0);
  });

  it('нет связи: плашка с повтором', async () => {
    signedIn();
    server.use(http.get('/api/v1/customers', () => HttpResponse.error()));
    renderApp('/queue');
    expect(await screen.findByText(/Нет связи с сервером\./)).toBeInTheDocument();
  });

  it('нет прав на сервере (403): «Недостаточно прав»', async () => {
    signedIn();
    server.use(
      http.get('/api/v1/customers', () =>
        HttpResponse.json(
          { type: 'about:blank', title: 'Запрещено', status: 403 },
          { status: 403, headers: { 'Content-Type': 'application/problem+json' } },
        ),
      ),
    );
    renderApp('/queue');
    expect(await screen.findByRole('heading', { name: 'Недостаточно прав' })).toBeInTheDocument();
  });

  it('двойной клик по «Баланс»: сначала убывание, затем возрастание, строки в этом порядке', async () => {
    signedIn();
    const user = userEvent.setup();
    renderApp('/queue');
    await screen.findByRole('table', { name: 'Клиенты под риском оттока' });
    const firstEmail = () => within(screen.getAllByRole('row')[1]!).getAllByRole('link')[0]!;

    await user.click(screen.getByRole('button', { name: 'Баланс' }));
    await waitFor(() => expect(firstEmail()).toHaveTextContent('user-0001'));

    await user.click(screen.getByRole('button', { name: 'Баланс' }));
    await waitFor(() => expect(firstEmail()).toHaveTextContent('user-0005'));
  });

  it('полоса «удалит биллинг» не зависит от поиска', async () => {
    signedIn();
    const user = userEvent.setup();
    renderApp('/queue');
    await screen.findByRole('heading', { name: /Удалит биллинг в ближайшие 5 дней: 1/ });
    await user.type(screen.getByLabelText('Поиск по email'), '0005');
    await waitFor(() =>
      expect(screen.queryByText('user-0002@example.test', { selector: 'a' })).toBeNull(),
    );
    expect(
      screen.getByRole('heading', { name: /Удалит биллинг в ближайшие 5 дней: 1/ }),
    ).toBeInTheDocument();
  });

  it('сводка не загрузилась: часы браузера не подставляются — нет полосы, окно спасения «—»', async () => {
    signedIn();
    server.use(
      http.get('/api/v1/analytics/summary', () =>
        HttpResponse.json(
          { type: 'about:blank', title: 'Сбой', status: 500 },
          { status: 500, headers: { 'Content-Type': 'application/problem+json' } },
        ),
      ),
    );
    renderApp('/queue');
    const table = await screen.findByRole('table', { name: 'Клиенты под риском оттока' });
    expect(screen.queryByRole('heading', { name: /Удалит биллинг/ })).toBeNull();
    for (const cell of within(table).getAllByText('—', {
      selector: 'td[data-label="Окно спасения"]',
    })) {
      expect(cell).toBeInTheDocument();
    }
    expect(table.querySelectorAll('td[data-label="Окно спасения"]')).toHaveLength(
      within(table).getAllByText('—', { selector: 'td[data-label="Окно спасения"]' }).length,
    );
  });

  describe('постраничный переход', () => {
    const many = (total = 230) => {
      server.use(
        http.get('/api/v1/customers', ({ request }) => {
          const q = new URL(request.url).searchParams;
          const limit = Number(q.get('limit') ?? 200);
          const offset = Number(q.get('offset') ?? 0);
          const items = Array.from({ length: total }, (_, i) => ({
            id: i + 1,
            email: `c${i + 1}@example.test`,
            balance: 1000,
            totalReplenished: 5000,
            score: 60,
            level: 'high',
            expectedLossCents: 100,
            topReason: 'мало денег',
          }));
          return HttpResponse.json({ items: items.slice(offset, offset + limit), total });
        }),
      );
    };

    it('230 клиентов: страница 5 показывает строку 201+ и подпись диапазона', async () => {
      signedIn();
      many();
      const user = userEvent.setup();
      const { router } = renderApp('/queue');
      expect(await screen.findByText(/Показаны 1–50 из 230/)).toBeInTheDocument();
      await user.click(screen.getByRole('button', { name: 'Страница 5' }));
      expect(await screen.findByText(/Показаны 201–230 из 230/)).toBeInTheDocument();
      expect(await screen.findByText('c201@example.test')).toBeInTheDocument();
      expect(router.state.location.search).toMatchObject({ page: 5 });
    });

    it('живое обновление перезапрашивает текущую страницу и не сбрасывает её', async () => {
      signedIn();
      many();
      const { router, queryClient } = renderApp('/queue?page=3');
      await screen.findByText(/Показаны 101–150 из 230/);
      await queryClient.invalidateQueries({ queryKey: customersKey });
      expect(await screen.findByText(/Показаны 101–150 из 230/)).toBeInTheDocument();
      expect(router.state.location.search).toMatchObject({ page: 3 });
    });

    it('одна страница: переключателя нет', async () => {
      signedIn();
      many(50);
      renderApp('/queue');
      await screen.findByText('c50@example.test');
      expect(screen.queryByText(/Показаны/)).toBeNull();
    });

    it('смена сортировки возвращает на первую страницу', async () => {
      signedIn();
      many();
      const user = userEvent.setup();
      const { router } = renderApp('/queue?page=3');
      await screen.findByText(/Показаны 101–150 из 230/);
      await user.click(screen.getByRole('button', { name: 'Email' }));
      await waitFor(() => expect(router.state.location.search).not.toHaveProperty('page'));
      expect(await screen.findByText(/Показаны 1–50 из 230/)).toBeInTheDocument();
    });

    it('страница за пределами total — первая', async () => {
      signedIn();
      many();
      const { router } = renderApp('/queue?page=99');
      await waitFor(() => expect(router.state.location.search).not.toHaveProperty('page'));
      expect(await screen.findByText(/Показаны 1–50 из 230/)).toBeInTheDocument();
    });
  });
});

describe('очередь: группа «У биллинга»', () => {
  it('клиент к биллингу не в таблице очереди, а в свёрнутой группе', async () => {
    signedIn();
    const user = userEvent.setup();
    const seen: Array<string | null> = [];
    server.events.on('request:start', ({ request }) => {
      const u = new URL(request.url);
      if (u.pathname === '/api/v1/customers') seen.push(u.searchParams.get('billing'));
    });
    renderApp('/queue');
    const table = await screen.findByRole('table', { name: 'Клиенты под риском оттока' });
    expect(within(table).queryByText(/user-0003/)).not.toBeInTheDocument();
    const control = await screen.findByRole('button', { name: /У биллинга \(1\)/ });
    expect(control).toHaveAttribute('aria-expanded', 'false');
    await user.click(control);
    const group = await screen.findByRole('table', { name: 'Клиенты у биллинга' });
    expect(within(group).getAllByRole('row')).toHaveLength(2);
    expect(seen).toContain('exclude');
    expect(seen).toContain('only');
  });

  it('нет таких клиентов: группы нет', async () => {
    signedIn();
    server.use(http.get('/api/v1/customers', () => HttpResponse.json({ items: [], total: 0 })));
    renderApp('/queue');
    await screen.findByRole('heading', { name: 'Сегодня некого спасать' });
    expect(screen.queryByText(/У биллинга/)).not.toBeInTheDocument();
  });

  it('ошибка группы: заголовок с повтором, очередь на месте', async () => {
    signedIn();
    server.use(
      http.get('/api/v1/customers', ({ request }) => {
        if (new URL(request.url).searchParams.get('billing') !== 'only') return undefined;
        return HttpResponse.json(
          { type: 'about:blank', title: 'Сбой', status: 500 },
          { status: 500, headers: { 'Content-Type': 'application/problem+json' } },
        );
      }),
    );
    renderApp('/queue');
    await screen.findByRole('table', { name: 'Клиенты под риском оттока' });
    expect(await screen.findByText('У биллинга — не загрузилось')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Повторить' })).toBeInTheDocument();
  });
});

describe('очередь: фильтр по работе', () => {
  it('«Без работы» уходит в запрос task=none и пишется в адрес', async () => {
    signedIn();
    const user = userEvent.setup();
    const seen: string[] = [];
    server.use(
      http.get('/api/v1/customers', ({ request }) => {
        seen.push(new URL(request.url).searchParams.get('task') ?? '');
        return HttpResponse.json({ items: [], total: 0 });
      }),
    );
    const { router } = renderApp('/queue');
    await screen.findByRole('radio', { name: 'Без работы' });
    await user.click(screen.getByRole('radio', { name: 'Без работы' }));
    await waitFor(() => expect(router.state.location.search).toMatchObject({ task: 'none' }));
    await waitFor(() => expect(seen).toContain('none'));
  });

  it('клиент в работе уходит из «Без работы» и total уменьшается', async () => {
    signedIn();
    sessionStorage.setItem('mock-session', 'marketer');
    seedTask(2);
    const user = userEvent.setup();
    renderApp('/queue');
    await screen.findByRole('table', { name: 'Клиенты под риском оттока' });
    const ids = () =>
      screen.getAllByRole('row').map((r) => within(r).queryAllByRole('cell')[0]?.textContent);
    expect(ids()).toContain('2');
    await user.click(screen.getByRole('radio', { name: 'Без работы' }));
    await waitFor(() => expect(ids()).not.toContain('2'));
  });
});
