import { createFileRoute } from '@tanstack/react-router';
import { QueuePage } from '@pages/queue';
import { RightGate } from '../layout/RightGate';

export const Route = createFileRoute('/_app/queue')({
  component: Section,
});

function Section() {
  const { user } = Route.useRouteContext();
  return (
    <RightGate role={user.role} right="queue.view" section="Очередь">
      <QueuePage />
    </RightGate>
  );
}
