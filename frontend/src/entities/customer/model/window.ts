import type { CustomerRow } from '../api/customers';

const DAY = 86_400_000;
const WAIT_DAYS = 5;

/** Идут 5 дней ожидания: срок прошёл, биллинг ещё не удалил услугу. Время — время стенда. */
export function inWaitWindow(row: CustomerRow, asOf: string): boolean {
  if (!row.expiresAt) return false;
  const now = Date.parse(asOf);
  const expires = Date.parse(row.expiresAt);
  return now >= expires && now < expires + WAIT_DAYS * DAY;
}

/** Когда биллинг удалит услугу, мс; для сортировки полосы «скоро удалит». */
export function deletionAt(row: CustomerRow): number {
  return row.expiresAt ? Date.parse(row.expiresAt) + WAIT_DAYS * DAY : Infinity;
}
