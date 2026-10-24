import { Alert, Button, Center, Skeleton, Stack, Text, Title } from '@mantine/core';
import type { ReactNode } from 'react';
import { IconAlertTriangle, IconInbox, IconLock, IconWifiOff } from '../icon';

/** Пять состояний экрана: загрузка, пусто, ошибка, нет связи, нет прав. */

export function StateLoading({ label = 'Загрузка' }: { label?: string }) {
  return (
    <Stack aria-busy="true" aria-label={label} role="status" gap="sm">
      <Skeleton height={28} width="40%" />
      <Skeleton height={16} />
      <Skeleton height={16} width="85%" />
      <Skeleton height={16} width="70%" />
    </Stack>
  );
}

function Message({
  icon,
  title,
  children,
  action,
}: {
  icon: ReactNode;
  title: string;
  children?: ReactNode;
  action?: ReactNode;
}) {
  return (
    <Center py="xl">
      <Stack align="center" gap="xs" maw={440} ta="center">
        {icon}
        <Title order={3}>{title}</Title>
        {children ? <Text c="dimmed">{children}</Text> : null}
        {action}
      </Stack>
    </Center>
  );
}

export function StateEmpty({
  title,
  children,
  action,
}: {
  title: string;
  children?: ReactNode;
  action?: ReactNode;
}) {
  return (
    <Message icon={<IconInbox size={32} aria-hidden />} title={title} action={action}>
      {children}
    </Message>
  );
}

export function StateError({
  title,
  children,
  requestId,
  onRetry,
}: {
  title: string;
  children?: ReactNode;
  requestId?: string | null;
  onRetry?: () => void;
}) {
  return (
    <Message
      icon={<IconAlertTriangle size={32} aria-hidden />}
      title={title}
      action={
        <Stack align="center" gap={4}>
          {onRetry ? <Button onClick={onRetry}>Повторить</Button> : null}
          {requestId ? (
            <Text size="xs" c="dimmed">
              Код запроса: {requestId}
            </Text>
          ) : null}
        </Stack>
      }
    >
      {children}
    </Message>
  );
}

/** Плашка над контентом: данные под ней остаются на экране. */
export function StateOffline({ onRetry }: { onRetry?: () => void }) {
  return (
    <Alert icon={<IconWifiOff size={18} aria-hidden />} color="orange" role="alert">
      Нет связи с сервером.{' '}
      {onRetry ? (
        <Button variant="subtle" size="compact-sm" onClick={onRetry}>
          Повторить сейчас
        </Button>
      ) : null}
    </Alert>
  );
}

export function StateForbidden({ children, action }: { children?: ReactNode; action?: ReactNode }) {
  return (
    <Message icon={<IconLock size={32} aria-hidden />} title="Недостаточно прав" action={action}>
      {children}
    </Message>
  );
}
