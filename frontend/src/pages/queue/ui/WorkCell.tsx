import { Text } from '@mantine/core';
import type { CustomerRow } from '@entities/customer';
import { dueText, statusLabel } from '@entities/retention-task';
import classes from './QueueTable.module.css';

/**
 * Кто и до какого срока работает с клиентом. Нет открытой задачи — пусто.
 * Срок считается от времени стенда; просроченный — цветом внимания, флаг приходит с сервера.
 */
export function WorkCell({ task, asOf }: { task: CustomerRow['task']; asOf: string | null }) {
  if (!task) return null;
  const due = asOf ? dueText(task.dueAt, Date.parse(asOf)).text : null;
  return (
    <span className={classes.work}>
      <Text span size="sm" fw={600}>
        {statusLabel[task.status]}
      </Text>
      <Text span size="sm" c="dimmed">
        {task.assignee}
      </Text>
      {due ? (
        <Text
          span
          size="sm"
          c={task.overdue ? 'orange' : 'dimmed'}
          fw={task.overdue ? 600 : undefined}
        >
          {task.overdue && !due.startsWith('срок прошёл') ? 'срок прошёл' : due}
        </Text>
      ) : null}
    </span>
  );
}
