import { Button } from '@mantine/core';
import { Link } from '@tanstack/react-router';
import type { ReactNode } from 'react';
import { can, homePath, rightHolders, roleLabel, type Right, type Role } from '@entities/session';
import { StateForbidden } from '@shared/ui';

/** Раздел целиком: нет права — экран «Недостаточно прав». Сервер проверяет всё заново. */
export function RightGate({
  role,
  right,
  section,
  children,
}: {
  role: Role;
  right: Right;
  section: string;
  children: ReactNode;
}) {
  if (can(role, right)) return <>{children}</>;
  return (
    <StateForbidden
      action={
        <Button component={Link} to={homePath(role)}>
          На главную
        </Button>
      }
    >
      Ваша роль — {roleLabel[role]}. Раздел «{section}» доступен {rightHolders[right]}.
    </StateForbidden>
  );
}
