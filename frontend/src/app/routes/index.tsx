import { createFileRoute, redirect } from '@tanstack/react-router';
import { homePath, meOptions } from '@entities/session';

// Корень ведёт на стартовую страницу роли; без сессии охрана _app отправит на вход.
export const Route = createFileRoute('/')({
  beforeLoad: async ({ context }) => {
    const user = await context.queryClient.ensureQueryData(meOptions);
    throw redirect({ to: user ? homePath(user.role) : '/login', replace: true });
  },
});
