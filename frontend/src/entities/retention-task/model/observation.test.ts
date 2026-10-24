import type { components } from '@shared/api';
import { observations, suggestedOutcome } from './observation';

type Event = components['schemas']['TaskEvent'];

const ev = (id: number, kind: Event['kind'], text: string): Event => ({
  id,
  at: '2026-10-24T12:00:00Z',
  actor: kind === 'observed' ? 'system' : 'marketer',
  kind,
  text,
  changes: [],
});

describe('наблюдения биллинга', () => {
  const events = [
    ev(1, 'created', ''),
    ev(2, 'observed', 'Баланс пополнен на 600,00 €'),
    ev(3, 'contact', 'позвонил'),
    ev(4, 'observed', 'Услуга VPS-1 продлена до 01.12.2026'),
  ];

  it('отбирает только наблюдения, новые сверху', () => {
    expect(observations(events).map((e) => e.id)).toEqual([4, 2]);
  });

  it.each([
    ['продление', 'Услуга VPS-1 продлена до 01.12.2026', 'renewed'],
    ['автопродление', 'Включено автопродление услуги VPS-1', 'renewed'],
    ['пополнение', 'Баланс пополнен на 600,00 €', null],
    ['блокировка', 'Услуга VPS-1 заблокирована', null],
  ])('итог по наблюдению «%s»', (_name, text, want) => {
    expect(suggestedOutcome([ev(1, 'observed', text)])).toBe(want);
  });

  it('без наблюдений итог не предлагается', () => {
    expect(suggestedOutcome([ev(1, 'created', '')])).toBeNull();
  });
});
