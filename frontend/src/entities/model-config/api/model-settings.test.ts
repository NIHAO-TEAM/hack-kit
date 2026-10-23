import { resetModelSettingsMock } from '../../../mocks/model-settings';
import { previewSettings, saveSettings } from './model-settings';
import { ApiError } from '@shared/api';

beforeEach(resetModelSettingsMock);

describe('model-settings api на моках', () => {
  it('предпросмотр: noauto до 0,85 — сменит уровень 1 клиент, +200,69 €', async () => {
    const r = await previewSettings({ weights: { noauto: 0.85 } });
    expect(r).toMatchObject({ levelChanges: { 'medium->high': 1 }, expectedLossDeltaCents: 20069 });
    expect(r.customersTotal).toBe(1);
    expect(r.customers?.[0]).toMatchObject({ id: 9, levelFrom: 'medium', levelTo: 'high' });
  });

  it('предпросмотр ничего не сохраняет, сохранение растит версию', async () => {
    await previewSettings({ weights: { noauto: 0.85 } });
    const saved = await saveSettings({ weights: { noauto: 0.85 } });
    expect(saved.version).toBe(2);
    expect(saved.weights.noauto).toBe(0.85);
  });

  it('422 несёт ошибки по полям', async () => {
    const err = await saveSettings({ weights: { noauto: 1.5 } }).catch((e: unknown) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect((err as ApiError).status).toBe(422);
    expect((err as ApiError).fieldErrors['weights.noauto']).toMatch(/от 0 до 1/);
  });
});
