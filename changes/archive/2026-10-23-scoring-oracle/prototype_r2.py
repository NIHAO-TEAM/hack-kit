"""Раунд 2 Скептика: модель после правок автора (r1_author.md).
Средний порог 50, funds по A P1-8 (полные цены продлений в окне 7 дн + почасовое), R_hour=720,
окно [asOf, asOf+H), asOf явный. Состояния: lost (все услуги deleted/blocked), unscored (нет услуг)."""
import json, sys, calendar, copy, collections
from datetime import datetime, timedelta, timezone

W = dict(pwait=0.80, funds=0.70, noauto=0.55, fail=0.40, susp=0.35, flight=0.15)
LIVE = ('active', 'suspended', 'prolong_wait')
REF = datetime(2026, 10, 19, 15, tzinfo=timezone.utc)


def p(s):
    return datetime.fromisoformat(s.replace('Z', '+00:00'))


def add_term(v, term, n=1):
    if term == 'hour':
        return v + timedelta(hours=n)
    if term == 'week':
        return v + timedelta(weeks=n)
    m = {'month': 1, 'quarter_year': 3, 'half_year': 6, 'year': 12}[term] * n
    mi = v.year * 12 + v.month - 1 + m
    y, mo = divmod(mi, 12)
    mo += 1
    return v.replace(year=y, month=mo, day=min(v.day, calendar.monthrange(y, mo)[1]))


def charges(s, as_of, hours):
    """Списаний в [asOf, asOf+hours). prolong_wait — один долг. Просроченная — одно в asOf, дальше шаг от asOf."""
    if s['status'] == 'prolong_wait':
        return 1
    if s['paymentTerm'] == 'hour':
        return hours
    end = as_of + timedelta(hours=hours)
    exp = p(s['expiresAt'])
    base = exp if exp >= as_of else as_of
    n, k, t = 0, 0, base
    while t < end:
        n += 1
        k += 1
        t = add_term(base, s['paymentTerm'], k)
    return n


def run(d, as_of, w=W, thr=(70, 50), pwait_debt=True):
    acc = {a['id']: a for a in d['accounts']}
    by_acc = collections.defaultdict(list)
    for s in d['services']:
        by_acc[s['ownerId']].append(s)
    tx = collections.defaultdict(list)
    for t in d['transactions']:
        if t['type'] == 'prolong':
            tx[t['serviceId']].append(t['status'])
    out = {}
    for aid, a in acc.items():
        svc = by_acc.get(aid, [])
        live = [s for s in svc if s['status'] in LIVE]
        if not svc:
            out[aid] = dict(level='unscored', loss=0, exposure=0, lost=0)
            continue
        lost = sum(s['price'] * charges(dict(s, status='active'), as_of, 720) for s in svc if s['status'] not in LIVE)
        if not live:
            out[aid] = dict(level='lost', loss=0, exposure=0, lost=lost)
            continue
        have = a['balance'] + (a['bonusBalance'] if a['bonusState'] == 'unlocked' else 0)
        need = 0
        for s in live:
            if s['status'] in ('active', 'suspended'):
                need += s['price'] * charges(s, as_of, 168)
            elif pwait_debt:
                need += s['price']
        ffunds = max(0.0, (need - have) / need) if need else 0.0
        fflight = a['withdrawBalance'] / a['balance'] if a['balance'] else 0.0
        best, loss, expo, det = 0.0, 0.0, 0, []
        for s in live:
            f = {}
            exp = p(s['expiresAt'])
            if s['status'] == 'prolong_wait':
                left = (exp + timedelta(days=5) - as_of).total_seconds() / 86400
                f['pwait'] = 1.0 if left <= 0 else min(1.0, max(0.0, 1 - left / 5))
            f['funds'] = ffunds
            if not s['autoProlong']:
                dl = (exp - as_of).total_seconds() / 86400
                f['noauto'] = 0.0 if dl > 7 else min(1.0, 1 - dl / 7)
            pr = tx.get(s['id'], [])
            f['fail'] = (sum(x == 'not_performed' for x in pr) / len(pr)) if pr else 0.0
            if s['status'] == 'suspended':
                f['susp'] = 1.0
            f['flight'] = fflight
            q = 1.0
            for k, v in f.items():
                q *= 1 - w[k] * v
            P = 1 - q
            R = s['price'] * charges(s, as_of, 720)
            pr_c = round(P * R)  # округление до цента на услугу
            loss += pr_c
            expo += R
            best = max(best, P)
            det.append((s['id'], s['paymentTerm'], s['status'], round(P, 3), R, pr_c))
        sc = 100 * best
        lvl = 'high' if sc >= thr[0] else 'medium' if sc >= thr[1] else 'low'
        out[aid] = dict(score=round(sc, 2), level=lvl, loss=loss, exposure=expo, lost=lost, funds=round(ffunds, 3), need=need, have=have, det=det)
    return out


def agg(out):
    lv = collections.Counter(v['level'] for v in out.values())
    tl = sum(v['loss'] for v in out.values())
    ar = sum(v['exposure'] for v in out.values() if v['level'] in ('high', 'medium'))
    lost = sum(v['lost'] for v in out.values())
    return lv, tl, ar, lost


def show(name, d, as_of, **kw):
    out = run(d, as_of, **kw)
    lv, tl, ar, lost = agg(out)
    top = max(out.items(), key=lambda x: x[1]['loss'])
    share = 100 * top[1]['loss'] / tl if tl else 0
    print(f'== {name} asOf={as_of:%Y-%m-%dT%H:%MZ} {kw}')
    print(f'   уровни {dict(lv)}; ожидаемая потеря {tl/100:.2f}; под угрозой {ar/100:.2f}; уже потеряно {lost/100:.2f}; топ №{top[0]} {top[1]["loss"]/100:.2f} ({share:.1f}%)')
    for aid, v in sorted(out.items()):
        if 'score' in v:
            print(f'   №{aid}: {v["score"]} {v["level"]} loss {v["loss"]/100:.2f} expo {v["exposure"]/100:.2f} funds {v["funds"]} need {v["need"]} have {v["have"]}')
        else:
            print(f'   №{aid}: {v["level"]} lost {v["lost"]/100:.2f}')
    return out


if __name__ == '__main__':
    d = json.load(open(sys.argv[1]))
    o = show('ПРАВКИ АВТОРА', d, REF)
    show('без долга pwait в funds', d, REF, pwait_debt=False)
    show('порог 45 (старый)', d, REF, thr=(70, 45))
    for name, t in (('часы: проверка жюри 24.10 12:00Z', datetime(2026, 10, 24, 12, tzinfo=timezone.utc)),
                    ('часы: питч 25.10 13:00Z', datetime(2026, 10, 25, 13, tzinfo=timezone.utc)),
                    ('часы: max дат снимка', max([p(t['createdAt']) for t in d['transactions']] + [p(t['performedAt']) for t in d['transactions'] if t['performedAt']] + [p(s['createdAt']) for s in d['services']]))):
        show(name, d, t)
    # контрфактуалы
    d9 = copy.deepcopy(d)
    a9 = next(a for a in d9['accounts'] if a['id'] == 9)
    top_up = o[9]['need'] - o[9]['have']
    a9['balance'] += top_up
    o9 = run(d9, REF)
    print(f'№9 пополнить на {top_up/100:.2f}: {o[9]["loss"]/100:.2f} -> {o9[9]["loss"]/100:.2f} ({(o9[9]["loss"]-o[9]["loss"])/100:.2f}); score {o[9]["score"]} -> {o9[9]["score"]} {o9[9]["level"]}')
    d1 = copy.deepcopy(d)
    for s in d1['services']:
        if s['ownerId'] == 1:
            s['autoProlong'] = True
    o1 = run(d1, REF)
    print(f'№1 автопродление: score {o[1]["score"]} -> {o1[1]["score"]} {o1[1]["level"]}, loss {o[1]["loss"]/100:.2f} -> {o1[1]["loss"]/100:.2f} ({(o1[1]["loss"]-o[1]["loss"])/100:.2f})')
    w2 = dict(W, noauto=0.85)
    o5 = run(d, REF, w=w2)
    lv, tl, ar, _ = agg(o5)
    print(f'noauto 0.85: №1 {o5[1]["score"]} {o5[1]["level"]}; уровни {dict(lv)}; потеря {tl/100:.2f}; под угрозой {ar/100:.2f}')
    for aid in (1, 4, 5, 6, 9, 10):
        print(aid, o[aid]['det'])
