import { formatMoney } from '@shared/lib';

export function clients(n: number): string {
  const m10 = n % 10;
  const m100 = n % 100;
  const word =
    m10 === 1 && m100 !== 11
      ? 'клиент'
      : m10 >= 2 && m10 <= 4 && (m100 < 12 || m100 > 14)
        ? 'клиента'
        : 'клиентов';
  return `${n} ${word}`;
}

/** «В работе 1 клиент · 3 189,10 € из 3 599,07 € ожидаемой потери (89 %)» */
export function inWorkText(customers: number, lossInWork: number, totalLoss: number): string {
  const share = totalLoss > 0 ? ` (${Math.round((lossInWork / totalLoss) * 100)} %)` : '';
  return `В работе ${clients(customers)} · ${formatMoney(lossInWork)} из ${formatMoney(totalLoss)} ожидаемой потери${share}`;
}

/** «Высокий риск без работы: 2 клиента · 300,00 €» */
export function highWithoutTaskText(n: number, loss: number): string {
  return `Высокий риск без работы: ${clients(n)} · ${formatMoney(loss)}`;
}
