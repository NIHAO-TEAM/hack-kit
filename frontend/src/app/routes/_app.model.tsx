import { createFileRoute } from '@tanstack/react-router';
import { settingsQuery } from '@entities/model-config';
import { can } from '@entities/session';
import { ModelPage } from '@pages/model-settings';
import { RightGate } from '../layout/RightGate';

export const Route = createFileRoute('/_app/model')({
  // Прогрев кэша: настройки грузятся, пока рисуется оболочка. Маркетологу они не нужны.
  loader: ({ context }) => {
    if (can(context.user.role, 'model.view')) void context.queryClient.prefetchQuery(settingsQuery);
  },
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
