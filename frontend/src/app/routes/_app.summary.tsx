import { createFileRoute } from '@tanstack/react-router';
import { SummaryPage } from '@pages/summary';
import { RightGate } from '../layout/RightGate';

export const Route = createFileRoute('/_app/summary')({
  component: Section,
});

function Section() {
  const { user } = Route.useRouteContext();
  return (
    <RightGate role={user.role} right="summary.view" section="Сводка">
      <SummaryPage />
    </RightGate>
  );
}
