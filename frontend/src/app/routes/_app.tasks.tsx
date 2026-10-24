import { createFileRoute } from '@tanstack/react-router';
import { TasksPage } from '@pages/tasks';
import { RightGate } from '../layout/RightGate';

const filters = ['mine', 'open', 'overdue', 'closed'] as const;

/** Фильтр списка — в адресе: ссылка со «Сводки» открывает просроченные. */
export const Route = createFileRoute('/_app/tasks')({
  validateSearch: (search: Record<string, unknown>) => ({
    filter: filters.find((f) => f === search.filter),
  }),
  component: Section,
});

function Section() {
  const { user } = Route.useRouteContext();
  const { filter } = Route.useSearch();
  return (
    <RightGate role={user.role} right="tasks.view" section="Задачи">
      <TasksPage role={user.role} initialFilter={filter} />
    </RightGate>
  );
}
