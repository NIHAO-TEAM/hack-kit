import { createFileRoute } from '@tanstack/react-router';
import { AboutModelPage } from '@pages/about-model';
import { RightGate } from '../layout/RightGate';

export const Route = createFileRoute('/_app/about')({
  component: Section,
});

function Section() {
  const { user } = Route.useRouteContext();
  return (
    <RightGate role={user.role} right="summary.view" section="О модели">
      <AboutModelPage />
    </RightGate>
  );
}
