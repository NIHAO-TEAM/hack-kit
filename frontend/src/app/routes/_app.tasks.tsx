import { createFileRoute } from '@tanstack/react-router';
import { TasksPage } from '@pages/tasks';
import { RightGate } from '../layout/RightGate';

export const Route = createFileRoute('/_app/tasks')({
  component: Section,
});

function Section() {
  const { user } = Route.useRouteContext();
  return (
    <RightGate role={user.role} right="tasks.view" section="Задачи">
      <TasksPage role={user.role} />
    </RightGate>
  );
}
