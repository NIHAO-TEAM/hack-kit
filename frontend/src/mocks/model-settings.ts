import { HttpResponse, http } from 'msw';
import type { components } from '@shared/api';
import { defaultSettings } from '@entities/model-settings';

type ModelSettings = components['schemas']['ModelSettings'];
type Patch = components['schemas']['ModelSettingsPatch'];

const current = (): ModelSettings => ({ ...defaultSettings, asOfMode: 'snapshot', version: 1 });

let state = current();

/** Сброс состояния между тестами. */
export function resetModelSettingsMock(): void {
  state = current();
}

function merge(base: ModelSettings, patch: Patch): ModelSettings {
  return {
    ...base,
    ...patch,
    weights: { ...base.weights, ...patch.weights },
    thresholds: { ...base.thresholds, ...patch.thresholds },
  };
}

/** Проверка как на сервере: поле → сообщение. */
function validate(s: ModelSettings): { field: string; message: string }[] {
  const errors: { field: string; message: string }[] = [];
  for (const [id, w] of Object.entries(s.weights)) {
    if (w < 0 || w > 1)
      errors.push({ field: `weights.${id}`, message: 'Вес должен быть от 0 до 1' });
  }
  if (s.thresholds.high <= s.thresholds.medium) {
    errors.push({
      field: 'thresholds.high',
      message: 'Порог высокого уровня должен быть выше среднего',
    });
  }
  if (s.horizonDays < 1 || s.horizonDays > 365) {
    errors.push({ field: 'horizonDays', message: 'Горизонт — от 1 до 365 дней' });
  }
  return errors;
}

/** Условный пересчёт моков: чем сильнее сдвиг весов, тем больше клиентов меняют уровень. */
function dryRun(next: ModelSettings) {
  const shift = Object.keys(next.weights).reduce(
    (sum, id) => sum + (next.weights[id]! - (state.weights[id] ?? next.weights[id]!)),
    0,
  );
  const count = Math.round(Math.abs(shift) * 3.3);
  return {
    levelChanges: count > 0 ? { [shift > 0 ? 'medium->high' : 'high->medium']: count } : {},
    expectedLossDeltaCents: Math.round(shift * 66897),
  };
}

export const modelSettingsHandlers = [
  http.get('/api/v1/model/settings', () => HttpResponse.json(state)),
  http.patch('/api/v1/model/settings', async ({ request }) => {
    const patch = (await request.json()) as Patch;
    const next = merge(state, patch);
    const errors = validate(next);
    if (errors.length > 0) {
      return HttpResponse.json(
        { type: 'about:blank', title: 'Проверьте значения', status: 422, errors },
        { status: 422, headers: { 'Content-Type': 'application/problem+json' } },
      );
    }
    if (new URL(request.url).searchParams.get('dryRun') === 'true') {
      return HttpResponse.json(dryRun(next));
    }
    state = { ...next, version: state.version + 1 };
    return HttpResponse.json(state);
  }),
];
