import { Anchor } from '@mantine/core';
import { useRouter } from '@tanstack/react-router';
import type { MouseEvent, ReactNode } from 'react';

/** Ссылка на карточку клиента: обычный href (открывается в новой вкладке), клик — без перезагрузки. */
export function CustomerLink({ id, children }: { id: number; children: ReactNode }) {
  const router = useRouter();
  const href = `/queue/${id}`;
  const open = (e: MouseEvent<HTMLAnchorElement>) => {
    if (e.metaKey || e.ctrlKey || e.shiftKey || e.button !== 0) return;
    e.preventDefault();
    void router.navigate({ href });
  };
  return (
    <Anchor href={href} onClick={open} size="sm">
      {children}
    </Anchor>
  );
}
