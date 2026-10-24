import type { components } from '@shared/api';

type Event = components['schemas']['TaskEvent'];
type Outcome = components['schemas']['TaskOutcome'];

/** Подпись под наблюдениями: данные биллинга показывают «после», но не «благодаря». */
export const OBSERVATION_NOTE =
  'Это данные биллинга после контакта. Они не доказывают, что помог именно контакт.';

/** Наблюдения биллинга из истории задачи, новые сверху. */
export function observations(events: readonly Event[]): Event[] {
  return events.filter((e) => e.kind === 'observed').reverse();
}

/** Услугу продлили или включили автопродление: такие наблюдения предлагают итог «Продлил». */
export function suggestedOutcome(events: readonly Event[]): Outcome | null {
  const renewed = events.some(
    (e) =>
      e.kind === 'observed' &&
      (/ продлена до /.test(e.text) || e.text.startsWith('Включено автопродление')),
  );
  return renewed ? 'renewed' : null;
}
