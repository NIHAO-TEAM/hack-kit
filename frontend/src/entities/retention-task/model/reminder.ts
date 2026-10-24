import { isOpenTask } from '../api/tasks';
import type { RetentionTask } from '../api/task';

const MSK_OFFSET_MS = 3 * 3_600_000;
const DAY_MS = 24 * 3_600_000;

/** Конец сегодняшнего дня по Москве (UTC+3, без перехода), в миллисекундах. */
export function endOfMskDay(now: number): number {
  return Math.floor((now + MSK_OFFSET_MS) / DAY_MS) * DAY_MS + DAY_MS - MSK_OFFSET_MS;
}

/** Напоминание: открытые задачи со сроком до конца сегодняшнего дня или уже просроченные. */
export function dueSoon(tasks: readonly RetentionTask[], now: number): RetentionTask[] {
  const end = endOfMskDay(now);
  return tasks
    .filter((t) => isOpenTask(t) && new Date(t.dueAt).getTime() < end)
    .sort((a, b) => a.dueAt.localeCompare(b.dueAt));
}
