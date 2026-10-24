import { act, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { conflictOn, seedTask } from '../mocks/tasks';
import { renderApp } from './render-app';

const asRole = (login: string) => sessionStorage.setItem('mock-session', login);

async function openCard(id = 9) {
  const view = renderApp(`/queue/${id}`);
  await screen.findByRole('heading', { name: `Клиент ${id}` });
  return view;
}

const panel = () => screen.getByRole('region', { name: 'Работа с клиентом' });

describe('карточка: работа с клиентом', () => {
  beforeEach(() => asRole('marketer'));

  it('«Отметить: взято в работу» одним кликом, срок и ответственный не показываются', async () => {
    const user = userEvent.setup();
    await openCard();
    await user.click(await screen.findByRole('button', { name: 'Отметить: взято в работу' }));
    expect(await within(panel()).findByText('Взято в работу')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Отметить: взято в работу' })).toBeNull();
    expect(within(panel()).queryByText(/Ответственный/)).toBeNull();
    expect(within(panel()).queryByText(/срок/i)).toBeNull();
  });

  it('нет «Связались», «Отложить», срока, заметки и истории контактов', async () => {
    seedTask(9, { note: 'позвонить' });
    await openCard();
    await screen.findByRole('button', { name: 'Закрыть' });
    for (const name of ['Связались', 'Отложить до…', 'Сохранить']) {
      expect(screen.queryByRole('button', { name })).toBeNull();
    }
    expect(screen.queryByLabelText('Срок')).toBeNull();
    expect(screen.queryByRole('textbox', { name: 'Заметка' })).toBeNull();
    expect(screen.queryByRole('list', { name: 'История задачи' })).toBeNull();
  });

  it('«Закрыть» требует итог из трёх; после закрытия клиента можно отметить заново', async () => {
    const user = userEvent.setup();
    seedTask(9);
    await openCard();
    await user.click(await screen.findByRole('button', { name: 'Закрыть' }));
    expect(screen.getByRole('button', { name: 'Закрыть задачу' })).toBeDisabled();
    await user.click(screen.getByRole('combobox', { name: 'Итог' }));
    expect(screen.getAllByRole('option', { hidden: true }).map((o) => o.textContent)).toEqual([
      'Продлил',
      'Ушёл',
      'Не удалось связаться',
    ]);
    await user.click(await screen.findByRole('option', { name: 'Продлил', hidden: true }));
    await user.click(screen.getByRole('button', { name: 'Закрыть задачу' }));
    expect(
      await screen.findByRole('button', { name: 'Отметить: взято в работу' }),
    ).toBeInTheDocument();
  });

  it('409 при закрытии: сообщение с логином, «Показать новое» берёт чужую версию', async () => {
    const user = userEvent.setup();
    seedTask(9);
    conflictOn.add(9);
    await openCard();
    await user.click(await screen.findByRole('button', { name: 'Закрыть' }));
    await user.click(screen.getByRole('combobox', { name: 'Итог' }));
    await user.click(await screen.findByRole('option', { name: 'Ушёл', hidden: true }));
    await user.click(screen.getByRole('button', { name: 'Закрыть задачу' }));
    expect(await screen.findByText(/Отметку уже изменил jury/)).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Показать новое' }));
    expect(screen.queryByText(/Отметку уже изменил/)).toBeNull();
  });

  it('кнопка закрытия — цель касания крупная (size lg, от 44 px)', async () => {
    seedTask(9);
    await openCard();
    expect(await screen.findByRole('button', { name: 'Закрыть' })).toHaveAttribute(
      'data-size',
      'lg',
    );
  });

  it('SSE tasks: изменение на другом экране появляется в карточке без перезагрузки', async () => {
    const { queryClient } = await openCard();
    expect(
      await screen.findByRole('button', { name: 'Отметить: взято в работу' }),
    ).toBeInTheDocument();
    seedTask(9, { assignee: 'jury' });
    await act(async () => {
      for (const root of ['customers', 'customer', 'task']) {
        await queryClient.invalidateQueries({ queryKey: [root] });
      }
    });
    expect(await screen.findByText('Взято в работу')).toBeInTheDocument();
  });
});

describe('карточка: аналитик', () => {
  it('видит отметку, но без кнопок', async () => {
    asRole('analyst');
    seedTask(9, { assignee: 'marketer' });
    await openCard();
    expect(
      await within(await screen.findByRole('region', { name: 'Работа с клиентом' })).findByText(
        'Взято в работу',
      ),
    ).toBeInTheDocument();
    for (const name of ['Закрыть', 'Отметить: взято в работу']) {
      expect(screen.queryByRole('button', { name })).toBeNull();
    }
  });

  it('клиент не взят в работу: пояснение и без кнопки', async () => {
    asRole('analyst');
    await openCard();
    expect(await screen.findByText('Клиента в работу не брали.')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Отметить: взято в работу' })).toBeNull();
  });

  describe('наблюдения биллинга', () => {
    beforeEach(() => asRole('marketer'));

    const observed = (id: number, text: string) => ({
      id,
      at: new Date().toISOString(),
      actor: 'system',
      kind: 'observed' as const,
      text,
      changes: [],
    });

    it('раздел «После взятия в работу» с подписью «после, а не благодаря»', async () => {
      seedTask(9, { events: [observed(101, 'Баланс пополнен на 600,00 €')] });
      await openCard();
      const section = await screen.findByRole('region', { name: 'После взятия в работу' });
      expect(within(section).getByText(/Баланс пополнен на 600,00 €/)).toBeVisible();
      expect(
        within(section).getByText(
          'Это данные биллинга после контакта. Они не доказывают, что помог именно контакт.',
        ),
      ).toBeVisible();
    });

    it('без наблюдений раздела нет', async () => {
      seedTask(9);
      await openCard();
      await screen.findByRole('region', { name: 'Работа с клиентом' });
      expect(screen.queryByRole('region', { name: 'После взятия в работу' })).toBeNull();
    });

    it('продление в наблюдениях: итог «Продлил» предложен, закрытие всё равно кнопкой человека', async () => {
      const user = userEvent.setup();
      seedTask(9, { events: [observed(102, 'Услуга VPS-1 продлена до 01.12.2026')] });
      await openCard();
      await user.click(await screen.findByRole('button', { name: 'Закрыть' }));
      expect(screen.getByRole('combobox', { name: 'Итог' })).toHaveValue('Продлил');
      expect(screen.getByRole('button', { name: 'Закрыть задачу' })).toBeEnabled();
    });

    it('пополнение итог не предлагает', async () => {
      const user = userEvent.setup();
      seedTask(9, { events: [observed(103, 'Баланс пополнен на 600,00 €')] });
      await openCard();
      await user.click(await screen.findByRole('button', { name: 'Закрыть' }));
      expect(screen.getByRole('combobox', { name: 'Итог' })).toHaveValue('');
      expect(screen.getByRole('button', { name: 'Закрыть задачу' })).toBeDisabled();
    });
  });
});
