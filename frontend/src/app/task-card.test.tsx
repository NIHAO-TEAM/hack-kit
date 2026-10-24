import { act, fireEvent, screen, waitFor, within } from '@testing-library/react';
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

  it('«Взять в работу» одним кликом: статус «В работе», ответственный — текущий логин', async () => {
    const user = userEvent.setup();
    await openCard();
    await user.click(await screen.findByRole('button', { name: 'Взять в работу' }));
    expect(await within(panel()).findByText('В работе')).toBeInTheDocument();
    expect(within(panel()).getByText('Ответственный: marketer')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Взять в работу' })).toBeNull();
    expect(within(panel()).getByText(/взята в работу/)).toBeInTheDocument();
  });

  it('«Связались»: канал и текст попадают в историю, статус «Связались»', async () => {
    const user = userEvent.setup();
    seedTask(9);
    await openCard();
    await user.click(await screen.findByRole('button', { name: 'Связались' }));
    await user.click(screen.getByRole('combobox', { name: 'Канал' }));
    await user.click(await screen.findByRole('option', { name: 'Телефон', hidden: true }));
    await user.type(screen.getByRole('textbox', { name: 'Что сказали' }), 'обещал пополнить');
    await user.click(screen.getByRole('button', { name: 'Записать контакт' }));
    const history = await screen.findByRole('list', { name: 'История задачи' });
    expect(
      await within(history).findByText(/связались \(телефон\): обещал пополнить/),
    ).toBeVisible();
    expect(within(panel()).getAllByText('Связались').length).toBeGreaterThan(0);
  });

  it('«Отложить до…»: статус «Отложено», срок обновлён', async () => {
    const user = userEvent.setup();
    seedTask(9);
    await openCard();
    await user.click(await screen.findByRole('button', { name: 'Отложить до…' }));
    fireEvent.change(screen.getByLabelText('Новый срок'), {
      target: { value: '2030-01-02T10:00' },
    });
    await user.click(screen.getByRole('button', { name: 'Отложить' }));
    expect(await within(panel()).findByText('Отложено')).toBeInTheDocument();
  });

  it('«Закрыть» требует итог; после закрытия клиента можно взять в работу заново', async () => {
    const user = userEvent.setup();
    seedTask(9);
    await openCard();
    await user.click(await screen.findByRole('button', { name: 'Закрыть' }));
    expect(screen.getByRole('button', { name: 'Закрыть задачу' })).toBeDisabled();
    await user.click(screen.getByRole('combobox', { name: 'Итог' }));
    await user.click(await screen.findByRole('option', { name: 'Продлил', hidden: true }));
    await user.click(screen.getByRole('button', { name: 'Закрыть задачу' }));
    expect(await screen.findByRole('button', { name: 'Взять в работу' })).toBeInTheDocument();
  });

  it('просроченный срок: «срок прошёл … назад»', async () => {
    seedTask(9, { dueAt: new Date(Date.now() - 3 * 3_600_000).toISOString(), overdue: true });
    await openCard();
    expect(await screen.findByText('срок прошёл 3 ч назад')).toBeInTheDocument();
  });

  it('409: сообщение с логином, введённая заметка остаётся, «Показать новое» берёт чужую версию', async () => {
    const user = userEvent.setup();
    seedTask(9, { note: 'старая' });
    conflictOn.add(9);
    await openCard();
    const note = await screen.findByRole('textbox', { name: 'Заметка' });
    await user.clear(note);
    await user.type(note, 'моя заметка');
    await user.click(screen.getByRole('button', { name: 'Сохранить' }));
    expect(await screen.findByText(/Задачу уже изменил jury/)).toBeInTheDocument();
    expect(screen.getByRole('textbox', { name: 'Заметка' })).toHaveValue('моя заметка');
    await user.click(screen.getByRole('button', { name: 'Показать новое' }));
    expect(screen.queryByText(/Задачу уже изменил/)).toBeNull();
    expect(screen.getByRole('textbox', { name: 'Заметка' })).toHaveValue('моя заметка');
    await user.click(screen.getByRole('button', { name: 'Сохранить' }));
    await waitFor(() => expect(screen.getByRole('button', { name: 'Сохранить' })).toBeDisabled());
  });

  it('кнопки действий — цели касания крупные (size lg, от 44 px)', async () => {
    seedTask(9);
    await openCard();
    for (const name of ['Связались', 'Отложить до…', 'Закрыть']) {
      expect(await screen.findByRole('button', { name })).toHaveAttribute('data-size', 'lg');
    }
  });

  it('SSE tasks: изменение на другом экране появляется в карточке без перезагрузки', async () => {
    const { queryClient } = await openCard();
    expect(await screen.findByRole('button', { name: 'Взять в работу' })).toBeInTheDocument();
    seedTask(9, { assignee: 'jury' });
    await act(async () => {
      for (const root of ['customers', 'customer', 'task']) {
        await queryClient.invalidateQueries({ queryKey: [root] });
      }
    });
    expect(await screen.findByText('Ответственный: jury')).toBeInTheDocument();
  });
});

describe('карточка: аналитик', () => {
  it('видит задачу, но без кнопок', async () => {
    asRole('analyst');
    seedTask(9, { assignee: 'marketer', note: 'позвонить' });
    await openCard();
    expect(
      await within(await screen.findByRole('region', { name: 'Работа с клиентом' })).findByText(
        'Ответственный: marketer',
      ),
    ).toBeInTheDocument();
    for (const name of ['Связались', 'Отложить до…', 'Закрыть', 'Сохранить', 'Взять в работу']) {
      expect(screen.queryByRole('button', { name })).toBeNull();
    }
  });

  it('клиент без задачи: «Задачи по клиенту нет» и без кнопки', async () => {
    asRole('analyst');
    await openCard();
    expect(await screen.findByText('Задачи по клиенту нет.')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Взять в работу' })).toBeNull();
  });
});
