import { useQueryClient } from '@tanstack/react-query';
import { createFileRoute, redirect, useRouter } from '@tanstack/react-router';
import { guestLogin, homePath, login, meOptions, sessionKey } from '@entities/session';
import { LoginPage } from '@pages/login';

export const Route = createFileRoute('/login')({
  beforeLoad: async ({ context }) => {
    const user = await context.queryClient.ensureQueryData(meOptions);
    if (user) throw redirect({ to: homePath(user.role), replace: true });
  },
  component: LoginRoute,
});

function LoginRoute() {
  const queryClient = useQueryClient();
  const router = useRouter();
  return (
    <LoginPage
      onSubmit={async (credentials) => {
        const user = await login(credentials);
        queryClient.setQueryData(sessionKey, user);
        await router.navigate({ to: homePath(user.role), replace: true });
      }}
      onGuest={async () => {
        const user = await guestLogin();
        queryClient.setQueryData(sessionKey, user);
        await router.navigate({ to: homePath(user.role), replace: true });
      }}
    />
  );
}
