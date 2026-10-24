import {
  AppShell as MantineShell,
  Box,
  Burger,
  Button,
  Group,
  NavLink,
  Text,
  Tooltip,
} from '@mantine/core';
import { useDisclosure } from '@mantine/hooks';
import { notifications } from '@mantine/notifications';
import { useQueryClient } from '@tanstack/react-query';
import { Link, Outlet, useRouter, useRouterState } from '@tanstack/react-router';
import { can, logout, sessionKey, type Right } from '@entities/session';
import { IconExternalLink, IconLogout } from '@shared/ui';
import { RealtimeProvider } from '../realtime';
import { Route } from '../routes/_app';
import { ThemeSwitch } from './ThemeSwitch';

const sections: {
  to: '/queue' | '/summary' | '/model' | '/source' | '/about';
  label: string;
  right: Right;
}[] = [
  { to: '/queue', label: 'Очередь', right: 'queue.view' },
  { to: '/summary', label: 'Сводка', right: 'summary.view' },
  { to: '/model', label: 'Модель', right: 'model.view' },
  { to: '/source', label: 'Источник', right: 'source.view' },
  { to: '/about', label: 'О модели', right: 'summary.view' },
];

export function AppShell() {
  const { user } = Route.useRouteContext();
  const queryClient = useQueryClient();
  const router = useRouter();
  const pathname = useRouterState({ select: (s) => s.location.pathname });
  const [opened, { toggle, close }] = useDisclosure();

  async function exit() {
    try {
      await logout();
    } catch {
      // Сессия на сервере жива: остаёмся в панели и говорим об этом.
      notifications.show({
        color: 'red',
        title: 'Не удалось выйти',
        message: 'Сервер не завершил сессию. Попробуйте ещё раз.',
      });
      return;
    }
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
            <Text fw={700} size="lg" component="div">
              Контур удержания
            </Text>
          </Group>
          <Group wrap="nowrap">
            {can(user.role, 'source.view') && (
              <Tooltip label="Графики Grafana: пересчёты, очередь, источник">
                <Button
                  component="a"
                  href="/grafana/"
                  target="_blank"
                  rel="noopener"
                  aria-label="Мониторинг"
                  variant="default"
                  size="compact-md"
                  rightSection={<IconExternalLink size={16} aria-hidden />}
                >
                  <Box component="span" visibleFrom="sm">
                    Мониторинг
                  </Box>
                </Button>
              </Tooltip>
            )}
            <ThemeSwitch />
            <Text size="sm" visibleFrom="sm">
              {user.login}
            </Text>
            <Button
              variant="default"
              size="compact-md"
              leftSection={<IconLogout size={16} aria-hidden />}
              onClick={exit}
              aria-label="Выйти"
            >
              <Box component="span" visibleFrom="sm">
                Выйти
              </Box>
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
        <RealtimeProvider>
          <Outlet />
        </RealtimeProvider>
      </MantineShell.Main>
    </MantineShell>
  );
}
