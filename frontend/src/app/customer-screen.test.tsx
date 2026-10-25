import { act, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { HttpResponse, http } from 'msw';
import type { CustomerDetail } from '@entities/customer-detail';
import { renderApp } from './render-app';

beforeEach(() => {
  sessionStorage.setItem('mock-session', 'marketer');
});

describe('экран «Карточка клиента»', () => {
  it('клиент 9: потеря, главное действие с ценой и пояснение суммы', async () => {
    renderApp('/queue/9');
    expect(await screen.findByRole('heading', { name: 'Клиент 9' })).toBeInTheDocument();
    expect(screen.getAllByText(/3\s189,10\s€/).length).toBeGreaterThan(0);
    const table = screen.getByRole('table', { name: 'Что можно сделать' });
    expect(within(table).getByText(/Пополнить на 529,84/)).toBeInTheDocument();
    expect(within(table).getByText(/−1\s027,49\s€/)).toBeInTheDocument();
    expect(screen.getByText(/платежи на 30 дней × вероятность срыва/i)).toBeInTheDocument();
    expect(screen.getByText(/расчётное снижение ожидаемой потери/i)).toBeInTheDocument();
    expect(screen.queryByText(/сохранит/i)).not.toBeInTheDocument();
  });

  it('клиент 9: таблица действий — цена, «на каждый €», лучшее и «Всё вместе»', async () => {
    renderApp('/queue/9');
    const table = await screen.findByRole('table', { name: 'Что можно сделать' });
    const rows = within(table).getAllByRole('row');
    expect(within(rows[1]!).getByText(/Пополнить на 529,84/)).toBeInTheDocument();
    expect(within(rows[1]!).getAllByRole('cell')[1]).toHaveTextContent(/^529,84\s€$/);
    expect(within(rows[1]!).getByText(/1,94\s€/)).toBeInTheDocument();
    expect(within(rows[1]!).getByText('лучшее по эффекту')).toBeInTheDocument();
    expect(within(table).getByText('Всё вместе')).toBeInTheDocument();
    expect(within(table).getByText(/−1\s029,32\s€/)).toBeInTheDocument();
    expect(screen.getByText(/если клиент выполнит действие полностью/i)).toBeInTheDocument();
    expect(screen.getByText(/не окупаемость/i)).toBeInTheDocument();
  });

  it('в карточке нет цикла «взять в работу», кнопок закрытия и блока наблюдений (ADR-0004)', async () => {
    renderApp('/queue/9');
    await screen.findByRole('heading', { name: 'Клиент 9' });
    expect(screen.getByRole('heading', { name: 'Что сделать' })).toBeInTheDocument();
    for (const name of ['Отметить: взято в работу', 'Закрыть', 'Связались', 'Отложить до…']) {
      expect(screen.queryByRole('button', { name })).toBeNull();
    }
    expect(screen.queryByRole('region', { name: 'Работа с клиентом' })).toBeNull();
    expect(screen.queryByRole('region', { name: 'После взятия в работу' })).toBeNull();
  });

  it('клиент 9: скор в шапке «87 %», формулы нет', async () => {
    renderApp('/queue/9');
    expect(await screen.findByText('скор 87 %')).toBeInTheDocument();
    expect(screen.queryByText(/8814/)).toBeNull();
    await screen.findByRole('heading', { name: 'Причины риска' });
    expect(screen.queryByText(/P = 1/)).toBeNull();
  });

  it('карточка клиента 9: причины по услугам по убыванию потери, без весов и таблицы', async () => {
    renderApp('/queue/9');
    await screen.findByRole('heading', { name: 'Причины риска' }, { timeout: 5000 });
    const blocks = screen.getAllByTestId('risk-service');
    expect(blocks).toHaveLength(2);
    expect(blocks[0]).toHaveTextContent(/3\s181,71\s€ под угрозой/);
    expect(blocks[1]).toHaveTextContent(/7,39\s€ под угрозой/);
    expect(within(blocks[0]!).getByText('Баланса не хватит')).toBeInTheDocument();
    expect(within(blocks[0]!).getAllByText(/сильная|заметная|слабая/).length).toBeGreaterThan(0);
    expect(
      within(blocks[0]!).queryByText('Часть баланса доступна к выводу', { exact: false }),
    ).toBeNull();
    expect(screen.queryByRole('table', { name: /Признаки риска/ })).toBeNull();
    expect(screen.queryByText('Вклад')).toBeNull();
    expect(screen.getByRole('link', { name: 'Как считается риск' })).toHaveAttribute(
      'href',
      '/about',
    );
  });

  it('раскрытая услуга: признаки услуги есть, формулы нет', async () => {
    const user = userEvent.setup();
    renderApp('/queue/9');
    await screen.findByRole('heading', { name: 'Причины риска' }, { timeout: 5000 });
    const controls = document.querySelectorAll<HTMLElement>('[data-accordion-control]');
    await user.click(controls[1]!);
    expect(screen.getAllByText(/Вероятность непродления P = 0,52 \(52 %\)/)).toHaveLength(2);
    expect(screen.getAllByText(/ожидаемая потеря P·R/)[1]).toHaveTextContent(/3\s181,71\s€/);
    const [table] = screen.getAllByRole('table', { name: 'Признаки услуги' });
    expect(within(table!).getByText('funds')).toBeInTheDocument();
    expect(screen.queryByTestId('service-formula')).toBeNull();
    expect(screen.queryByText(/P = 1/)).toBeNull();
  });

  it('без serviceRisks причины клиента — одним списком без группировки', async () => {
    const { server } = await import('../mocks/server');
    const res = await fetch('/api/v1/customers/9');
    const detail = (await res.json()) as CustomerDetail;
    server.use(
      http.get('/api/v1/customers/9', () =>
        HttpResponse.json({ ...detail, serviceRisks: undefined }),
      ),
    );
    renderApp('/queue/9');
    await screen.findByRole('heading', { name: 'Причины риска' }, { timeout: 5000 });
    expect(screen.getByText('Автопродление выключено')).toBeInTheDocument();
    expect(screen.queryByTestId('risk-service')).toBeNull();
  });

  it('без действующих признаков — «Явных причин риска нет»', async () => {
    const { server } = await import('../mocks/server');
    const res = await fetch('/api/v1/customers/9');
    const detail = (await res.json()) as CustomerDetail;
    server.use(
      http.get('/api/v1/customers/9', () =>
        HttpResponse.json({ ...detail, serviceRisks: undefined, features: [] }),
      ),
    );
    renderApp('/queue/9');
    expect(
      await screen.findByText('Явных причин риска нет', undefined, { timeout: 5000 }),
    ).toBeInTheDocument();
  });

  it('клиент 9: сумма потерь по услугам сходится с шапкой', async () => {
    renderApp('/queue/9');
    expect(
      await screen.findByText(/Сумма потерь по услугам = ожидаемая потеря клиента/, undefined, {
        timeout: 5000,
      }),
    ).toHaveTextContent(/3\s189,10\s€/);
    expect(screen.queryByText('Сумма не сходится с шапкой')).toBeNull();
  });

  it('расхождение суммы услуг с шапкой видно, а не скрыто', async () => {
    const { server } = await import('../mocks/server');
    const res = await fetch('/api/v1/customers/9');
    const detail = (await res.json()) as CustomerDetail;
    server.use(
      http.get('/api/v1/customers/9', () =>
        HttpResponse.json({
          ...detail,
          customer: {
            ...detail.customer,
            expectedLossCents: detail.customer.expectedLossCents + 500,
          },
        }),
      ),
    );
    renderApp('/queue/9');
    expect(
      await screen.findByText('Сумма не сходится с шапкой', undefined, { timeout: 5000 }),
    ).toBeInTheDocument();
  });

  it('клиент 7: неживая услуга без вероятности, с пометкой', async () => {
    const user = userEvent.setup();
    renderApp('/queue/7');
    await screen.findByText(/потерян: живых услуг не осталось/, undefined, { timeout: 5000 });
    const controls = document.querySelectorAll<HTMLElement>('[data-accordion-control]');
    await user.click(controls[0]!);
    expect(screen.getAllByText(/в ожидаемую потерю не входит/).length).toBeGreaterThan(0);
    expect(screen.queryByText(/Вероятность непродления/)).toBeNull();
    expect(screen.queryByText(/Сумма потерь по услугам/)).toBeNull();
  });

  it('без serviceRisks: «Оценок по услугам нет», без сверки', async () => {
    const user = userEvent.setup();
    const { server } = await import('../mocks/server');
    const res = await fetch('/api/v1/customers/9');
    const detail = (await res.json()) as CustomerDetail;
    const rest = { ...detail, serviceRisks: undefined };
    server.use(http.get('/api/v1/customers/9', () => HttpResponse.json(rest)));
    renderApp('/queue/9');
    await screen.findByRole('heading', { name: 'Причины риска' }, { timeout: 5000 });
    await user.click(document.querySelector<HTMLElement>('[data-accordion-control]')!);
    expect(screen.getAllByText('Оценок по услугам нет.').length).toBeGreaterThan(0);
    expect(screen.queryByText(/Сумма потерь по услугам/)).toBeNull();
  });

  it('нечисловой адрес — «Такого клиента нет»', async () => {
    renderApp('/queue/abc');
    expect(await screen.findByText('Такого клиента нет')).toBeInTheDocument();
  });

  it('клиент 7: «потерян», сумма потерянных услуг, а не 0 €', async () => {
    renderApp('/queue/7');
    expect(await screen.findByText(/потерян: живых услуг не осталось/)).toBeInTheDocument();
    expect(
      screen.getByText(/Стоимость потерянных услуг за горизонт модели \(расчёт\)/),
    ).toBeInTheDocument();
    expect(screen.getAllByText(/6\s808,74\s€/).length).toBeGreaterThan(0);
    expect(screen.getByText(/не фактическая выручка/)).toBeInTheDocument();
  });

  it('скор карточки совпадает со скором очереди для каждого клиента мока', async () => {
    const { rows } = await import('../mocks/queue');
    const { server } = await import('../mocks/server');
    expect(server).toBeDefined();
    for (const row of rows()) {
      const res = await fetch(`/api/v1/customers/${row.id}`);
      const detail = (await res.json()) as CustomerDetail;
      expect(detail.customer.score ?? null, `клиент ${row.id}`).toBe(row.score ?? null);
    }
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

  it('новая версия пересчёта подсвечивает изменившиеся числа, без неё — нет', async () => {
    const { queryClient } = renderApp('/queue/9');
    await screen.findByRole('heading', { name: 'Клиент 9' });
    const key = ['customer', 9];
    const old = queryClient.getQueryData<CustomerDetail>(key)!;
    const lit = () => document.querySelectorAll('[data-changed="true"]').length;

    act(() => {
      queryClient.setQueryData(key, {
        ...old,
        customer: { ...old.customer, expectedLossCents: old.customer.expectedLossCents + 100 },
      });
    });
    expect(lit()).toBe(0);

    act(() => {
      queryClient.setQueryData(key, {
        ...old,
        version: old.version + 1,
        customer: { ...old.customer, expectedLossCents: old.customer.expectedLossCents + 500 },
      });
    });
    await waitFor(() => expect(lit()).toBeGreaterThan(0));
  });

  it('окно спасения считается от asOf сводки, а не от часов браузера', async () => {
    vi.useFakeTimers({ toFake: ['Date'], now: new Date('2030-01-01T00:00:00Z') });
    try {
      const { server } = await import('../mocks/server');
      server.use(
        http.get('/api/v1/analytics/summary', () =>
          HttpResponse.json({
            expectedLossCents: 0,
            atRiskCents: 0,
            lostCents: 0,
            asOf: '2026-10-19T15:00:00.000Z',
          }),
        ),
        http.get('/api/v1/customers/9', () =>
          HttpResponse.json({
            version: 1,
            customer: {
              id: 9,
              email: 'c9@example.com',
              balance: 0,
              totalReplenished: 0,
              score: 88,
              level: 'high',
              expectedLossCents: 100,
            },
            // срок вышел за 2 д 7 ч до asOf: идут 5 дней ожидания, до удаления 2 д 17 ч
            services: [
              {
                id: 1,
                ownerId: 9,
                name: 'VPS',
                paymentTerm: 'month',
                status: 'active',
                expiresAt: '2026-10-17T08:00:00.000Z',
              },
            ],
            features: [],
            actions: [],
          }),
        ),
      );
      renderApp('/queue/9');
      await screen.findByRole('heading', { name: 'Клиент 9' });
      expect((await screen.findAllByText(/до удаления 2\sд\s17\sч/)).length).toBeGreaterThan(0);
      expect(screen.queryByText('окно закрыто')).toBeNull();
    } finally {
      vi.useRealTimers();
    }
  });

  it('пока сводка не пришла — «время модели загружается» вместо полосы', async () => {
    const { server } = await import('../mocks/server');
    server.use(http.get('/api/v1/analytics/summary', () => new Promise(() => {})));
    renderApp('/queue/9');
    await screen.findByRole('heading', { name: 'Клиент 9' });
    expect(screen.getAllByText('время модели загружается').length).toBeGreaterThan(0);
  });

  it('сводка недоступна — «время модели недоступно», а не вечная загрузка', async () => {
    const { server } = await import('../mocks/server');
    server.use(http.get('/api/v1/analytics/summary', () => HttpResponse.error()));
    renderApp('/queue/9');
    await screen.findByRole('heading', { name: 'Клиент 9' });
    expect((await screen.findAllByText('время модели недоступно')).length).toBeGreaterThan(0);
  });
});
