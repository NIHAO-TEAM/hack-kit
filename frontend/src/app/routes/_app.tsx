import { createFileRoute, redirect } from '@tanstack/react-router';
import { meOptions } from '@entities/session';
import { AppShell } from '../layout/AppShell';

// Охрана: без сессии — на вход. Пользователь кладётся в контекст дочерних маршрутов.
export const Route = createFileRoute('/_app')({
  beforeLoad: async ({ context }) => {
    const user = await context.queryClient.ensureQueryData(meOptions);
    if (!user) throw redirect({ to: '/login', replace: true });
    return { user };
  },
  component: AppShell,
});
