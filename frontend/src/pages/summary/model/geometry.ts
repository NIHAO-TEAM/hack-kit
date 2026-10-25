import type { ChurnPoint } from '../api/analytics';

export interface XY {
  x: number;
  y: number;
}

export interface ChartGeometry {
  /** Линия факта: только недели с известным `actual`. */
  actual: XY[];
  /** Линия прогноза: только недели с известным `forecast`. */
  forecast: XY[];
  /** Позиция недели «сейчас» — последняя с фактом. */
  nowX: number | null;
  yMax: number;
  xOf: (i: number) => number;
  yOf: (v: number) => number;
}

export interface Box {
  width: number;
  height: number;
  left: number;
  right: number;
  top: number;
  bottom: number;
}

/** Круглый шаг сетки: 1, 2, 2,5 (от 10) или 5 × 10^k, не меньше `raw`; подписи целые. */
export function niceStep(raw: number): number {
  if (raw <= 1) return 1;
  const pow = 10 ** Math.floor(Math.log10(raw));
  const m = raw / pow;
  const mult = m <= 1 ? 1 : m <= 2 ? 2 : m <= 2.5 && pow >= 10 ? 2.5 : m <= 5 ? 5 : 10;
  return mult * pow;
}

/** Верх шкалы — четыре круглых деления: подписи вида 25 000, а не 12 345,75. */
export function niceMax(max: number): number {
  return 4 * niceStep(max / 4);
}

/** Переводит недели в координаты SVG. Чистая функция: тестируется без DOM. */
export function layout(points: ChurnPoint[], box: Box): ChartGeometry {
  const max = Math.max(0, ...points.flatMap((p) => [p.actual ?? 0, p.forecast ?? 0]));
  const yMax = niceMax(max);
  const innerW = box.width - box.left - box.right;
  const innerH = box.height - box.top - box.bottom;
  const xOf = (i: number) =>
    box.left + (points.length > 1 ? (innerW * i) / (points.length - 1) : innerW / 2);
  const yOf = (v: number) => box.top + innerH - (innerH * v) / yMax;
  const actual: XY[] = [];
  const forecast: XY[] = [];
  let nowX: number | null = null;
  points.forEach((p, i) => {
    if (p.actual !== null) {
      actual.push({ x: xOf(i), y: yOf(p.actual) });
      nowX = xOf(i);
    }
    if (p.forecast !== null) forecast.push({ x: xOf(i), y: yOf(p.forecast) });
  });
  return { actual, forecast, nowX, yMax, xOf, yOf };
}

export const path = (pts: XY[]): string =>
  pts.map((p, i) => `${i === 0 ? 'M' : 'L'}${p.x.toFixed(1)} ${p.y.toFixed(1)}`).join(' ');
