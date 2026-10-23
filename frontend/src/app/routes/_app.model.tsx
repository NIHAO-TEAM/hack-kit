import { createFileRoute } from '@tanstack/react-router';
import { ModelPage } from '@pages/model-settings';
import { RightGate } from '../layout/RightGate';

export const Route = createFileRoute('/_app/model')({
  component: Section,
});

function Section() {
  const { user } = Route.useRouteContext();
  return (
    <RightGate role={user.role} right="model.view" section="Модель">
      <ModelPage />
    </RightGate>
  );
}
