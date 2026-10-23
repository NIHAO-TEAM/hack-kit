const NBSP = ' ';
const MINUS = '−';

/** Центы → «3 879,73 €» (неразрывные пробелы, десятичная запятая, минус U+2212). */
export function formatMoney(cents: number): string {
  const abs = Math.abs(Math.round(cents));
  const euros = String(Math.floor(abs / 100)).replace(/\B(?=(\d{3})+(?!\d))/g, NBSP);
  const rest = String(abs % 100).padStart(2, '0');
  return `${cents < 0 ? MINUS : ''}${euros},${rest}${NBSP}€`;
}

const pad = (n: number) => String(n).padStart(2, '0');

/** Остаток времени: «2 д 07 ч», «7 ч 05 мин», «меньше минуты». Отрицательный и нулевой — «меньше минуты». */
export function formatCountdown(ms: number): string {
  const totalMin = Math.max(0, Math.floor(ms / 60_000));
  const days = Math.floor(totalMin / 1440);
  const hours = Math.floor((totalMin % 1440) / 60);
  const minutes = totalMin % 60;
  if (days > 0) return `${days}${NBSP}д${NBSP}${pad(hours)}${NBSP}ч`;
  if (hours > 0) return `${hours}${NBSP}ч${NBSP}${pad(minutes)}${NBSP}мин`;
  if (minutes > 0) return `${minutes}${NBSP}мин`;
  return 'меньше минуты';
}
