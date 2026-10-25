import type { ServiceRisk } from '../api/customer';
import { activeReasons, byContribution, riskReasons, scorePercentText } from './features';

const features = [
  { id: 'susp', value: 1, weight: 0.35, contribution: 0.35, reason: 'Услуга приостановлена' },
  { id: 'funds', value: 1, weight: 0.7, contribution: 0.7, reason: '' },
  { id: 'flight', value: 1, weight: 0, contribution: 0, reason: 'Вывод' },
];

const title = (id: string) => `название ${id}`;

const risk = (serviceId: number, lossCents: number): ServiceRisk => ({
  serviceId,
  probability: 0.5,
  rCents: lossCents * 2,
  lossCents,
  features,
  quality: [],
});

describe('признаки карточки', () => {
  it('сортируются по убыванию вклада и не меняют исходный массив', () => {
    expect(byContribution(features).map((f) => f.id)).toEqual(['funds', 'susp', 'flight']);
    expect(features[0]?.id).toBe('susp');
  });

  it('скор в процентах: шапка целое', () => {
    expect(scorePercentText(88.14)).toBe('скор 88 %');
  });
});

describe('причины риска', () => {
  it('без нулевого вклада, по убыванию силы; пустой текст заменяет название признака', () => {
    const reasons = activeReasons(features, title);
    expect(reasons.map((r) => r.id)).toEqual(['funds', 'susp']);
    expect(reasons[0]).toMatchObject({ text: 'название funds', strength: 'strong' });
    expect(reasons[1]).toMatchObject({ text: 'Услуга приостановлена', strength: 'noticeable' });
  });

  it('сила по доле вклада: слабая меньше 15 %', () => {
    const f = [
      { id: 'a', value: 1, weight: 1, contribution: 0.9, reason: 'A' },
      { id: 'b', value: 1, weight: 1, contribution: 0.05, reason: 'B' },
    ];
    expect(activeReasons(f, title).map((r) => r.strength)).toEqual(['strong', 'weak']);
  });

  it('услуги по убыванию потери, без нулевой, доли в сумме дают 1', () => {
    const list = riskReasons([risk(1, 100), risk(2, 0), risk(3, 300)], title);
    expect(list.map((s) => s.serviceId)).toEqual([3, 1]);
    expect(list[0]?.share).toBeCloseTo(0.75);
    expect(list.reduce((sum, s) => sum + s.share, 0)).toBeCloseTo(1);
  });

  it('нет услуг с потерей — пустой список', () => {
    expect(riskReasons([risk(1, 0)], title)).toEqual([]);
    expect(riskReasons([], title)).toEqual([]);
  });
});
