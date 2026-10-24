"""Эталонный расчёт модели риска оттока. Правила — docs/model-rules.md.

Только стандартная библиотека, Python 3.10+. Запуск: python3 -I tools/oracle/oracle.py ...
  oracle.py <snapshot.json> --as-of <ISO> [--settings <settings.json>]   JSON в stdout
  oracle.py <snapshot.json> --as-of <ISO> --per-customer                по строке JSON на клиента (сверка с Go)
  oracle.py --check                                                     сверить все golden
  oracle.py --regen                                                     пересчитать golden
  oracle.py <snapshot.json> --as-of <ISO> --sensitivity                 чувствительность к весам ±20 %
"""
import argparse
import calendar
import collections
import copy
import glob
import json
import math
import os
import sys
from datetime import datetime, timedelta, timezone

ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
GOLDEN_DIR = os.path.join(ROOT, 'testdata', 'golden')
LIVE = ('active', 'suspended', 'prolong_wait')
DEFAULTS = {
    'weights': {'pwait': 0.80, 'funds': 0.70, 'noauto': 0.55, 'fail': 0.40, 'susp': 0.35, 'flight': 0.0, 'bonus_locked': 0.0},
    'high': 70,
    'medium': 50,
    'horizonHours': 720,
    'fundsHours': 168,
    'priceBasis': 'per_period',
}


def parse(s):
    return datetime.fromisoformat(s.replace('Z', '+00:00'))


def fmt(t):
    return t.astimezone(timezone.utc).strftime('%Y-%m-%dT%H:%M:%SZ')


def add_term(v, term, n):
    if term == 'hour':
        return v + timedelta(hours=n)
    if term == 'week':
        return v + timedelta(weeks=n)
    m = {'month': 1, 'quarter_year': 3, 'half_year': 6, 'year': 12}[term] * n
    y, mo = divmod(v.year * 12 + v.month - 1 + m, 12)
    mo += 1
    return v.replace(year=y, month=mo, day=min(v.day, calendar.monthrange(y, mo)[1]))


def charges(s, as_of, hours):
    """Число списаний услуги в окне [asOf, asOf + hours)."""
    if s['status'] == 'prolong_wait':
        return 1
    if s['paymentTerm'] == 'hour':
        return hours
    end = as_of + timedelta(hours=hours)
    exp = parse(s['expiresAt'])
    base = exp if exp >= as_of else as_of
    n = 0
    while add_term(base, s['paymentTerm'], n) < end:
        n += 1
    return n


def amount(s, as_of, hours, st):
    """Деньги по услуге за окно в центах: price × списания; per_month у hour — price за 720 ч."""
    if s['paymentTerm'] == 'hour' and st['priceBasis'] == 'per_month' and s['status'] != 'prolong_wait':
        return round(s['price'] * hours / 720)
    return s['price'] * charges(s, as_of, hours)


def merge_settings(over):
    st = copy.deepcopy(DEFAULTS)
    over = over or {}
    for k, v in over.items():
        if k == 'weights':
            st['weights'].update(v)
        else:
            st[k] = v
    return st


def customer_funds(a, live, as_of, st):
    have = a['balance'] + (a['bonusBalance'] if a['bonusState'] == 'unlocked' else 0)
    need = 0
    for s in live:
        if s['status'] == 'prolong_wait':
            need += s['price']
        else:
            need += amount(s, as_of, st['fundsHours'], st)
    return need, have


def allocate(loss, f, w):
    """Потеря услуги по признакам пропорционально w·f; округление вниз, остаток — самому большому вкладу."""
    contrib = {k: w[k] * v for k, v in sorted(f.items())}
    total = 0.0
    for v in contrib.values():
        total += v
    if loss <= 0 or total <= 0:
        return {}
    out = {k: int(math.floor(loss * c / total)) for k, c in contrib.items()}
    top = min(contrib, key=lambda k: (-contrib[k], k))
    out[top] += loss - sum(out.values())
    return {k: v for k, v in out.items() if v}


def score_customer(a, svc, tx, as_of, st):
    live = [s for s in svc if s['status'] in LIVE]
    if not svc:
        return {'state': 'unscored', 'score': None, 'level': None, 'expectedLossCents': 0, 'exposureCents': 0, 'lostCents': 0, 'features': {}}
    lost = sum(amount(dict(s, status='active'), as_of, st['horizonHours'], st) for s in svc if s['status'] not in LIVE)
    if not live:
        return {'state': 'lost', 'score': None, 'level': None, 'expectedLossCents': 0, 'exposureCents': 0, 'lostCents': lost, 'features': {}}
    need, have = customer_funds(a, live, as_of, st)
    ffunds = max(0.0, (need - have) / need) if need else 0.0
    fflight = a['withdrawBalance'] / a['balance'] if a['balance'] else 0.0
    bonus_locked = a['bonusBalance'] > 0 and a['bonusState'] == 'locked' and have < need
    best, loss, expo, feats, lbf = 0.0, 0, 0, {}, {}
    for s in sorted(live, key=lambda x: x['id']):
        exp = parse(s['expiresAt'])
        f = {}
        if s['status'] == 'prolong_wait':
            left = (exp + timedelta(days=5) - as_of).total_seconds() / 86400
            f['pwait'] = 1.0 if left <= 0 else min(1.0, max(0.0, 1 - left / 5))
        f['funds'] = ffunds
        if not s['autoProlong']:
            days = (exp - as_of).total_seconds() / 86400
            f['noauto'] = 0.0 if days > 7 else min(1.0, 1 - days / 7)
        pr = tx.get(s['id'], [])
        f['fail'] = sum(x == 'not_performed' for x in pr) / len(pr) if pr else 0.0
        if s['status'] == 'suspended':
            f['susp'] = 1.0
        f['flight'] = fflight
        if bonus_locked:
            f['bonus_locked'] = 1.0
        q = 1.0
        for k, v in f.items():
            q *= 1 - st['weights'][k] * v
        P = 1 - q
        R = amount(s, as_of, st['horizonHours'], st)
        sl = round(P * R)
        loss += sl
        for k, v in allocate(sl, f, st['weights']).items():
            lbf[k] = lbf.get(k, 0) + v
        expo += R
        best = max(best, P)
        feats[str(s['id'])] = {k: round(v, 6) for k, v in f.items()}
    sc = round(100 * best, 2)
    level = 'high' if sc >= st['high'] else 'medium' if sc >= st['medium'] else 'low'
    return {'state': 'scored', 'score': sc, 'level': level, 'expectedLossCents': loss, 'exposureCents': expo, 'lostCents': lost,
            'features': feats, 'lossByFeature': lbf, '_need': need, '_have': have}


def prepare(snap):
    by_acc = collections.defaultdict(list)
    for s in snap['services']:
        by_acc[s['ownerId']].append(s)
    tx = collections.defaultdict(list)
    for t in snap['transactions']:
        if t['type'] == 'prolong':
            tx[t['serviceId']].append(t['status'])
    return by_acc, tx


def calc_all(snap, as_of, st):
    by_acc, tx = prepare(snap)
    return {a['id']: score_customer(a, by_acc.get(a['id'], []), tx, as_of, st) for a in snap['accounts']}


def actions(snap, as_of, st, base):
    """Действия с ценой: пополнить / включить автопродление; эффект = потеря после − потеря до."""
    by_acc, tx = prepare(snap)
    out = {}
    for a in snap['accounts']:
        r = base[a['id']]
        if r['state'] != 'scored':
            out[a['id']] = []
            continue
        svc = by_acc.get(a['id'], [])
        acts = []
        feats = r['features']

        def effect(kind, amount_cents, a2, svc2):
            r2 = score_customer(a2, svc2, tx, as_of, st)
            d = {'type': kind, 'lossDeltaCents': r2['expectedLossCents'] - r['expectedLossCents'],
                 'scoreBefore': r['score'], 'scoreAfter': r2['score'], 'levelAfter': r2['level']}
            if amount_cents is not None:
                d['amountCents'] = amount_cents
            return d

        if any(f['funds'] > 0 for f in feats.values()):
            top = r['_need'] - r['_have']
            acts.append(effect('top_up', top, dict(a, balance=a['balance'] + top), svc))
        if any(f.get('noauto', 0) > 0 for f in feats.values()):
            svc2 = [dict(s, autoProlong=True) for s in svc]
            acts.append(effect('enable_autoprolong', None, a, svc2))
        out[a['id']] = acts
    return out


def compute(snap, as_of, settings=None):
    st = merge_settings(settings)
    res = calc_all(snap, as_of, st)
    acts = actions(snap, as_of, st, res)
    customers = []
    for aid in sorted(res):
        r = res[aid]
        c = {'id': aid, 'state': r['state'], 'score': r['score'], 'level': r['level'],
             'expectedLossCents': r['expectedLossCents'], 'exposureCents': r['exposureCents'], 'lostCents': r['lostCents'],
             'features': r['features'], 'lossByFeature': r.get('lossByFeature', {}), 'actions': acts[aid]}
        customers.append(c)
    lv = collections.Counter(c['level'] or c['state'] for c in customers)
    by_feature = {}
    for c in customers:
        for k, v in c['lossByFeature'].items():
            e = by_feature.setdefault(k, {'id': k, 'lossCents': 0, 'customers': 0})
            e['lossCents'] += v
            e['customers'] += 1
    summary = {
        'lossByFeature': sorted(by_feature.values(), key=lambda e: (-e['lossCents'], e['id'])),
        'levels': {k: lv.get(k, 0) for k in ('high', 'medium', 'low', 'lost', 'unscored')},
        'expectedLossCents': sum(c['expectedLossCents'] for c in customers),
        'exposureCents': sum(c['exposureCents'] for c in customers if c['level'] in ('high', 'medium')),
        'lostCents': sum(c['lostCents'] for c in customers),
    }
    return {'asOf': fmt(as_of), 'settings': st, 'customers': customers, 'summary': summary}


SHIFT = 0.2
TOP_N = 3


def sensitivity(snap, as_of, settings=None):
    """Сдвиг веса каждого признака с ненулевым весом на −20 % и +20 %: сколько клиентов сменили уровень,
    изменилась ли первая тройка по потере, на сколько сдвинулась суммарная потеря. Без выборки."""
    st = merge_settings(settings)
    base = calc_all(snap, as_of, st)

    def top(res):
        scored = [(-r['expectedLossCents'], aid) for aid, r in res.items() if r['state'] == 'scored']
        return [aid for _, aid in sorted(scored)[:TOP_N]]

    def shifted(feature, factor):
        s2 = copy.deepcopy(st)
        s2['weights'][feature] = min(1.0, max(0.0, st['weights'][feature] * factor))
        res = calc_all(snap, as_of, s2)
        return {
            'levelChanges': sum(1 for aid, r in res.items() if r['level'] is not None and r['level'] != base[aid]['level']),
            'topThreeChanged': top(res) != top(base),
            'lossDeltaCents': sum(r['expectedLossCents'] for r in res.values()) - sum(r['expectedLossCents'] for r in base.values()),
        }

    feats = []
    for k in sorted(st['weights']):
        w = st['weights'][k]
        if w == 0:
            continue
        feats.append({'id': k, 'weight': w, 'down': shifted(k, 1 - SHIFT), 'up': shifted(k, 1 + SHIFT)})
    return {'asOf': fmt(as_of), 'customers': len(base), 'features': feats, 'sampled': False, 'settings': st}


def sensitivity_files():
    return sorted(glob.glob(os.path.join(GOLDEN_DIR, 'sensitivity', '*.json')))


def build_sensitivity(path):
    g = json.load(open(path, encoding='utf-8'))
    snap = json.load(open(os.path.join(ROOT, g['source']), encoding='utf-8'))
    return dict(sensitivity(snap, parse(g['asOf']), g.get('settings')), source=g['source'])


def dump(obj):
    return json.dumps(obj, ensure_ascii=False, indent=2, sort_keys=True) + '\n'


def golden_files():
    return sorted(p for p in glob.glob(os.path.join(GOLDEN_DIR, '*.json')) if not p.endswith('.snapshot.json'))


def build_from_golden(path):
    g = json.load(open(path, encoding='utf-8'))
    snap = json.load(open(os.path.join(ROOT, g['source']), encoding='utf-8'))
    out = compute(snap, parse(g['asOf']), g['settings'])
    return g, dict(out, source=g['source'])


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('snapshot', nargs='?')
    ap.add_argument('--as-of')
    ap.add_argument('--settings')
    ap.add_argument('--per-customer', action='store_true')
    ap.add_argument('--check', action='store_true')
    ap.add_argument('--regen', action='store_true')
    ap.add_argument('--sensitivity', action='store_true')
    args = ap.parse_args()
    if args.check or args.regen:
        bad = 0
        for p in golden_files():
            g, new = build_from_golden(p)
            if args.regen:
                open(p, 'w', encoding='utf-8').write(dump(new))
                print('записан', os.path.relpath(p, ROOT))
            elif dump(new) != open(p, encoding='utf-8').read():
                bad += 1
                print('РАСХОЖДЕНИЕ', os.path.relpath(p, ROOT), file=sys.stderr)
        for p in sensitivity_files():
            new = build_sensitivity(p)
            if args.regen:
                open(p, 'w', encoding='utf-8').write(dump(new))
                print('записан', os.path.relpath(p, ROOT))
            elif dump(new) != open(p, encoding='utf-8').read():
                bad += 1
                print('РАСХОЖДЕНИЕ', os.path.relpath(p, ROOT), file=sys.stderr)
        if args.check:
            print('golden: %d, расхождений: %d' % (len(golden_files()) + len(sensitivity_files()), bad))
        return 1 if bad else 0
    if not args.snapshot or not args.as_of:
        ap.error('нужны snapshot и --as-of')
    snap = json.load(open(args.snapshot, encoding='utf-8'))
    st = json.load(open(args.settings, encoding='utf-8')) if args.settings else None
    if args.sensitivity:
        sys.stdout.write(dump(sensitivity(snap, parse(args.as_of), st)))
        return 0
    out = compute(snap, parse(args.as_of), st)
    if args.per_customer:
        for c in out['customers']:
            acts = {a['type']: a['lossDeltaCents'] for a in c['actions']}
            best = min(acts, key=lambda t: (acts[t], t)) if acts else None
            sys.stdout.write(json.dumps({
                'id': c['id'], 'state': c['state'], 'level': c['level'], 'score': c['score'],
                'expectedLossCents': c['expectedLossCents'], 'exposureCents': c['exposureCents'],
                'lostCents': c['lostCents'], 'actions': acts, 'best': best}, sort_keys=True) + '\n')
        return 0
    sys.stdout.write(dump(out))
    return 0


if __name__ == '__main__':
    sys.exit(main())
