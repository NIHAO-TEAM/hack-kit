import { inWaitWindow } from './window';
import type { CustomerRow } from '../api/customers';

const AS_OF = '2026-10-19T15:00:00.000Z';
const row = (rescueExpiresAt?: string): CustomerRow => ({
  id: 1,
  email: 'a@example.test',
  balance: 0,
  totalReplenished: 0,
  level: 'high',
  expectedLossCents: 0,
  rescueExpiresAt,
});

describe('inWaitWindow', () => {
  it('срок прошёл, 5 дней не вышли — в окне', () => {
    expect(inWaitWindow(row('2026-10-18T15:00:00.000Z'), AS_OF)).toBe(true);
  });
  it('срок ещё впереди — не в окне', () => {
    expect(inWaitWindow(row('2026-10-19T16:00:00.000Z'), AS_OF)).toBe(false);
  });
  it('окно закрыто (прошло больше 5 дней) — не в окне', () => {
    expect(inWaitWindow(row('2026-10-10T15:00:00.000Z'), AS_OF)).toBe(false);
  });
  it('без срока — не в окне', () => {
    expect(inWaitWindow(row(), AS_OF)).toBe(false);
  });
});
