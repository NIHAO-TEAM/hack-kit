import type { StreamState } from '../api/stream';

/** Доля выполненного прогона, 0–100. */
export function progressPercent(state: Pick<StreamState, 'applied' | 'events'>): number {
  if (state.events <= 0) return 0;
  return Math.min(100, Math.round((state.applied / state.events) * 100));
}

const NBSP = ' ';

/** Число с пробелами между тысячами: «2 500». */
export function formatCount(n: number): string {
  return String(Math.round(n)).replace(/\B(?=(\d{3})+(?!\d))/g, NBSP);
}

/** Миллисекунды: «0,8 мс», «18,5 мс», «1 240 мс». */
export function formatMs(ms: number): string {
  if (ms >= 1000) return `${formatCount(ms)}${NBSP}мс`;
  const digits = ms < 10 ? 2 : 1;
  return `${ms.toFixed(digits).replace('.', ',')}${NBSP}мс`;
}

/** Длительность прогона: «47,8 с», «1 мин 05 с». */
export function formatDuration(ms: number): string {
  const sec = ms / 1000;
  if (sec < 60) return `${sec.toFixed(1).replace('.', ',')}${NBSP}с`;
  const minutes = Math.floor(sec / 60);
  const rest = Math.round(sec % 60);
  return `${minutes}${NBSP}мин ${String(rest).padStart(2, '0')}${NBSP}с`;
}

/** События в секунду: «52,4 /с». */
export function formatRate(perSecond: number): string {
  return `${perSecond.toFixed(1).replace('.', ',')}${NBSP}/с`;
}
