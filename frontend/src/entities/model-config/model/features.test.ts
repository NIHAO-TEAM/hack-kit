import { readdirSync, readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { featureHint, featureInfo } from './features';

/** Признаки, которые считает backend: `ID()` в `backend/internal/risk/feature_*.go`. */
function backendFeatureIds(): string[] {
  const dir = resolve(__dirname, '../../../../../backend/internal/risk');
  return readdirSync(dir)
    .filter((f) => /^feature_.+\.go$/.test(f) && !f.endsWith('_test.go'))
    .map(
      (f) =>
        /ID\(\) string\s*\{ return "(\w+)" \}/.exec(readFileSync(resolve(dir, f), 'utf8'))?.[1],
    )
    .filter((id): id is string => Boolean(id));
}

describe('featureInfo', () => {
  it('у каждого признака backend есть подпись', () => {
    const ids = backendFeatureIds();
    expect(ids).toContain('bonus_locked');
    for (const id of ids) {
      expect(featureInfo[id], `нет подписи признака ${id}`).toBeDefined();
    }
  });

  it('подсказка не утверждает долю исхода и называет вес экспертной оценкой', () => {
    expect(featureHint('pwait')).not.toContain('%');
    expect(featureHint('pwait')).toContain('экспертная оценка');
  });
});
