import { createFileRoute } from '@tanstack/react-router';
import { SourcePage } from '@pages/source';
import { RightGate } from '../layout/RightGate';

export const Route = createFileRoute('/_app/source')({
  component: Section,
});

function Section() {
  const { user } = Route.useRouteContext();
  return (
    <RightGate role={user.role} right="source.view" section="Источник">
      <SourcePage />
    </RightGate>
  );
}
