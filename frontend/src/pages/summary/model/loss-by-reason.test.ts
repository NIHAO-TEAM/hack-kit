import { formatShare, reasonRows } from './loss-by-reason';

describe('потеря по причинам', () => {
  it('подписи причин берутся из словаря признаков, неизвестный признак остаётся своим id', () => {
    const rows = reasonRows([
      { featureId: 'funds', lossCents: 134738, customers: 1, sharePercent: 37.44 },
      { featureId: 'mystery', lossCents: 100, customers: 1, sharePercent: 0.03 },
    ]);
    expect(rows[0]?.title).toBe('Не хватает средств на продление');
    expect(rows[1]?.title).toBe('mystery');
  });

  it('доля с запятой и неразрывным пробелом, ширина полосы ограничена 0..100', () => {
    expect(formatShare(51.24)).toBe('51,24 %'.replace(' ', String.fromCharCode(160)));
    expect(
      reasonRows([{ featureId: 'fail', lossCents: 1, customers: 1, sharePercent: 130 }])[0]?.width,
    ).toBe(100);
  });
});
