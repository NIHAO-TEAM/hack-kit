import { act, screen, waitFor, within } from '@testing-library/react';
import { TASK_ROOT } from '@entities/retention-task';
import { resetTasks, seedTask } from '../mocks/tasks';
import { renderApp } from './render-app';

const asRole = (login: string) => sessionStorage.setItem('mock-session', login);
const HOUR = 3_600_000;
const inHours = (h: number) => new Date(Date.now() + h * HOUR).toISOString();

describe('экран «Задачи»', () => {
  beforeEach(() => resetTasks());

  it.each(['marketer', 'analyst', 'jury'])('пункт «Задачи» в меню: %s', async (role) => {
    asRole(role);
    renderApp('/tasks');
    const nav = await screen.findByRole('navigation', { name: 'Разделы' });
    expect(within(nav).getByText('Задачи')).toBeInTheDocument();
  });

  it('у маркетолога по умолчанию «Мои»: чужие задачи не видны, срок словами', async () => {
    asRole('marketer');
    seedTask(9, { dueAt: inHours(-2), overdue: true, customerEmail: 'a@x.test' });
    seedTask(10, { assignee: 'jury', customerEmail: 'b@x.test' });
    renderApp('/tasks');
    expect(await screen.findByText(/срок прошёл 2 ч назад/)).toBeInTheDocument();
    expect(screen.getByRole('radio', { name: 'Мои' })).toBeChecked();
    expect(screen.queryByText(/b@x.test/)).toBeNull();
    expect(screen.getByRole('link', { name: /9 · a@x.test/ })).toHaveAttribute('href', '/queue/9');
  });

  it('у аналитика по умолчанию «Все открытые»', async () => {
    asRole('analyst');
    seedTask(10, { assignee: 'jury', customerEmail: 'b@x.test' });
    renderApp('/tasks');
    expect(await screen.findByText(/b@x.test/)).toBeInTheDocument();
    expect(screen.getByRole('radio', { name: 'Все открытые' })).toBeChecked();
  });

  it('клиента нет в снимке: пояснение без ссылки', async () => {
    asRole('marketer');
    seedTask(9, { customerMissing: true, customerEmail: 'gone@x.test' });
    renderApp('/tasks');
    expect(await screen.findByText('клиента нет в текущем снимке')).toBeInTheDocument();
    expect(screen.queryByRole('link', { name: /gone@x.test/ })).toBeNull();
  });

  it('пусто: объясняет, что делать', async () => {
    asRole('marketer');
    renderApp('/tasks');
    expect(
      await screen.findByText('Задач нет. Возьмите клиента в работу из очереди или карточки.'),
    ).toBeInTheDocument();
  });
});

describe('напоминание о сроках в шапке', () => {
  beforeEach(() => resetTasks());

  it('считает просроченные и сегодняшние; закрытая задача уходит после события tasks', async () => {
    asRole('marketer');
    const overdue = seedTask(9, { dueAt: inHours(-3), overdue: true });
    seedTask(10, { dueAt: inHours(-1), overdue: true });
    seedTask(11, { dueAt: inHours(24 * 5) });
    const { queryClient } = renderApp('/queue');
    expect(await screen.findByRole('button', { name: 'Сроки: 2' })).toBeInTheDocument();
    overdue.status = 'closed';
    await act(() => queryClient.invalidateQueries({ queryKey: [TASK_ROOT] }));
    await waitFor(() => expect(screen.getByRole('button', { name: 'Сроки: 1' })).toBeVisible());
  });

  it('у аналитика значка нет', async () => {
    asRole('analyst');
    seedTask(9, { dueAt: inHours(-3), overdue: true });
    renderApp('/model');
    await screen.findByRole('navigation', { name: 'Разделы' });
    expect(screen.queryByRole('button', { name: /Сроки/ })).toBeNull();
  });
});
