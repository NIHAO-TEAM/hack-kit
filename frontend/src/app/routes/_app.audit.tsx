import { createFileRoute } from '@tanstack/react-router';
import { AuditPage } from '@pages/audit';
import { RightGate } from '../layout/RightGate';

export const Route = createFileRoute('/_app/audit')({
  component: Section,
});

function Section() {
  const { user } = Route.useRouteContext();
  return (
    <RightGate role={user.role} right="model.view" section="Журнал">
      <AuditPage />
    </RightGate>
  );
}
