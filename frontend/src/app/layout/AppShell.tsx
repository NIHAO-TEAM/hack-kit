import {
  AppShell as MantineShell,
  Burger,
  Button,
  Group,
  NavLink,
  Text,
  Title,
} from '@mantine/core';
import { useDisclosure } from '@mantine/hooks';
import { useQueryClient } from '@tanstack/react-query';
import { Link, Outlet, useRouter, useRouterState } from '@tanstack/react-router';
import { can, logout, sessionKey, type Right } from '@entities/session';
import { IconLogout } from '@shared/ui';
import { Route } from '../routes/_app';

const sections: {
  to: '/queue' | '/summary' | '/model' | '/source';
  label: string;
  right: Right;
}[] = [
  { to: '/queue', label: 'Очередь', right: 'queue.view' },
  { to: '/summary', label: 'Сводка', right: 'summary.view' },
  { to: '/model', label: 'Модель', right: 'model.view' },
  { to: '/source', label: 'Источник', right: 'source.view' },
];

export function AppShell() {
  const { user } = Route.useRouteContext();
  const queryClient = useQueryClient();
  const router = useRouter();
  const pathname = useRouterState({ select: (s) => s.location.pathname });
  const [opened, { toggle, close }] = useDisclosure();

  async function exit() {
    await logout();
    // Сессия сброшена в кэше: кнопка «назад» упрётся в охрану и вернёт на вход.
    queryClient.setQueryData(sessionKey, null);
    queryClient.removeQueries({ predicate: (q) => q.queryKey[0] !== sessionKey[0] });
    await router.navigate({ to: '/login', replace: true });
  }

  return (
    <MantineShell
      header={{ height: 56 }}
      navbar={{ width: 220, breakpoint: 'sm', collapsed: { mobile: !opened } }}
      padding="md"
    >
      <MantineShell.Header>
        <Group h="100%" px="md" justify="space-between" wrap="nowrap">
          <Group wrap="nowrap">
            <Burger opened={opened} onClick={toggle} hiddenFrom="sm" size="sm" aria-label="Меню" />
            <Title order={1} size="h4">
              Контур удержания
            </Title>
          </Group>
          <Group wrap="nowrap">
            <Text size="sm">{user.login}</Text>
            <Button
              variant="default"
              size="compact-md"
              leftSection={<IconLogout size={16} aria-hidden />}
              onClick={exit}
            >
              Выйти
            </Button>
          </Group>
        </Group>
      </MantineShell.Header>
      <MantineShell.Navbar p="xs" component="nav" aria-label="Разделы">
        {sections
          .filter((s) => can(user.role, s.right))
          .map((s) => (
            <NavLink
              key={s.to}
              component={Link}
              to={s.to}
              label={s.label}
              active={pathname.startsWith(s.to)}
              onClick={close}
            />
          ))}
      </MantineShell.Navbar>
      <MantineShell.Main>
        <Outlet />
      </MantineShell.Main>
    </MantineShell>
  );
}
