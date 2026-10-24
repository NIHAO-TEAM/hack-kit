import type { StreamWindowPoint } from '@entities/stream-run';

/** Окно графиков — последние 60 с прогона. */
export const WINDOW_SECONDS = 60;

export interface XY {
  x: number;
  y: number;
}

/** Верх шкалы: ближайшее «круглое» число 1, 2, 5 × 10ⁿ не меньше максимума. */
export function niceMax(max: number): number {
  if (max <= 1) return 1;
  const exp = 10 ** Math.floor(Math.log10(max));
  for (const m of [1, 2, 5, 10]) if (m * exp >= max) return m * exp;
  return 10 * exp;
}

/** Точки линии в координатах SVG: по оси X — секунда в окне 60 с, по Y — значение от нуля до yMax. */
export function linePoints(
  values: number[],
  box: { width: number; height: number },
  yMax: number,
): XY[] {
  const step = box.width / (WINDOW_SECONDS - 1);
  const offset = WINDOW_SECONDS - values.length;
  return values.map((v, i) => ({
    x: (offset + i) * step,
    y: box.height - (Math.min(v, yMax) / yMax) * box.height,
  }));
}

export function pathOf(points: XY[]): string {
  return points
    .map((p, i) => `${i === 0 ? 'M' : 'L'}${p.x.toFixed(1)} ${p.y.toFixed(1)}`)
    .join(' ');
}

export type SeriesKey = 'rate' | 'queue' | 'batch' | 'lag' | 'recalcs' | 'failed';

export interface Series {
  key: SeriesKey;
  title: string;
  unit: string;
  /** Линии графика: первая — главная; у «событий в секунду» их две. */
  lines: { label: string; values: number[] }[];
}

/** Шесть рядов панели из секундных отсчётов; только пересчёт формы, цифры — из прогона. */
export function seriesOf(window: StreamWindowPoint[]): Series[] {
  const pick = (f: (p: StreamWindowPoint) => number) => window.map(f);
  return [
    {
      key: 'rate',
      title: 'События в секунду',
      unit: 'соб./с',
      lines: [
        { label: 'принято', values: pick((p) => p.accepted) },
        { label: 'применено', values: pick((p) => p.applied) },
      ],
    },
    {
      key: 'queue',
      title: 'Очередь приёма',
      unit: 'соб.',
      lines: [{ label: 'ждут записи', values: pick((p) => p.queueDepth) }],
    },
    {
      key: 'batch',
      title: 'Размер пачки',
      unit: 'соб.',
      lines: [{ label: 'в среднем', values: pick((p) => p.batchSize) }],
    },
    {
      key: 'lag',
      title: 'Задержка до очереди',
      unit: 'мс',
      lines: [{ label: 'p95', values: pick((p) => p.lagP95Ms) }],
    },
    {
      key: 'recalcs',
      title: 'Пересчётов в секунду',
      unit: 'перес./с',
      lines: [{ label: 'выполнено', values: pick((p) => p.recalcs) }],
    },
    {
      key: 'failed',
      title: 'Ошибки',
      unit: 'соб.',
      lines: [{ label: 'не применено', values: pick((p) => p.failed) }],
    },
  ];
}

/** Склонение: plural(1, ['цент','цента','центов']) → «цент». */
export function plural(n: number, forms: [string, string, string]): string {
  const m10 = n % 10;
  const m100 = n % 100;
  if (m10 === 1 && m100 !== 11) return forms[0];
  if (m10 >= 2 && m10 <= 4 && (m100 < 12 || m100 > 14)) return forms[1];
  return forms[2];
}

/** Итоговая строка прогона: «Ошибок 0 · потерянных 0 · согласованность 0 центов». */
export function totalsLine(r: {
  events: number;
  applied: number;
  failed: number;
  consistency: { diffCents: number };
}): string {
  const lost = Math.max(0, r.events - r.applied - r.failed);
  const cents = r.consistency.diffCents;
  return `Ошибок ${r.failed} · потерянных ${lost} · согласованность ${cents} ${plural(cents, ['цент', 'цента', 'центов'])}`;
}
