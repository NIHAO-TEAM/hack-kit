import type { components } from '@shared/api';
import { formatMoney } from '@shared/lib';

type DryRunResult = components['schemas']['DryRunResult'];

/** Сколько клиентов сменят уровень (ключи вида «medium->high»). */
export function changedCount(result: DryRunResult): number {
  return Object.values(result.levelChanges).reduce((sum, n) => sum + n, 0);
}

function plural(n: number): string {
  const m10 = n % 10;
  const m100 = n % 100;
  if (m10 === 1 && m100 !== 11) return 'клиент';
  if (m10 >= 2 && m10 <= 4 && (m100 < 12 || m100 > 14)) return 'клиента';
  return 'клиентов';
}

/** «сменят уровень 1 клиент, потеря +200,69 €» */
export function dryRunText(result: DryRunResult): string {
  const n = changedCount(result);
  const delta = result.expectedLossDeltaCents;
  const money = `${delta > 0 ? '+' : ''}${formatMoney(delta)}`;
  return `сменят уровень ${n} ${plural(n)}, потеря ${money}`;
}
