import { Button, Popover, Stack, Text } from '@mantine/core';
import { useQuery } from '@tanstack/react-query';
import { useEffect, useState } from 'react';
import { CustomerLink, dueSoon, dueText, tasksQuery } from '@entities/retention-task';

/**
 * Напоминание в шапке: мои открытые задачи со сроком до конца сегодняшнего дня и просроченные.
 * Перечитывается по SSE `tasks` (общий корень ключей) и раз в минуту: срок наступает и без событий.
 */
export function TaskReminder() {
  const tasks = useQuery({ ...tasksQuery('mine'), refetchInterval: 60_000 });
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const timer = setInterval(() => setNow(Date.now()), 60_000);
    return () => clearInterval(timer);
  }, []);
  const due = dueSoon(tasks.data?.items ?? [], now);
  if (due.length === 0) return null;
  return (
    <Popover position="bottom-end" withArrow shadow="md">
      <Popover.Target>
        <Button variant="default" size="compact-md" aria-label={`Сроки: ${due.length}`}>
          Сроки: {due.length}
        </Button>
      </Popover.Target>
      <Popover.Dropdown>
        <Stack gap="xs" component="ul" m={0} p={0} style={{ listStyle: 'none' }}>
          {due.map((t) => {
            const text = dueText(t.dueAt, now);
            return (
              <li key={t.id}>
                {t.customerMissing ? (
                  <Text size="sm">Клиент {t.customerId}</Text>
                ) : (
                  <CustomerLink id={t.customerId}>Клиент {t.customerId}</CustomerLink>
                )}
                <Text size="xs" c={text.overdue ? 'var(--lost-fg)' : 'dimmed'}>
                  {text.text}
                </Text>
              </li>
            );
          })}
        </Stack>
      </Popover.Dropdown>
    </Popover>
  );
}
