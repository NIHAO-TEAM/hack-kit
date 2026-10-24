"""Построчное сравнение расчёта Go (snapcalc) и эталона (oracle.py --per-customer).

  parity_compare.py go.jsonl oracle.jsonl   код 1 при расхождении

Сверяются состояние, уровень, скор, деньги в центах и набор действий с ценой (тип → эффект) и лучшее действие, точно; порядок списка действий не сверяется. В вывод — число клиентов,
число расхождений и первые 20 расхождений с id клиента. Только стандартная библиотека.
"""
import json
import sys

SHOW = 20


def lines(path):
    with open(path, encoding='utf-8') as f:
        for ln in f:
            yield json.loads(ln)


def main():
    if len(sys.argv) != 3:
        print(__doc__, file=sys.stderr)
        return 2
    n = bad = 0
    shown = 0
    go_it, or_it = lines(sys.argv[1]), lines(sys.argv[2])
    for g, o in zip(go_it, or_it):
        n += 1
        if g != o:
            bad += 1
            if shown < SHOW:
                shown += 1
                diff = {k: (g.get(k), o.get(k)) for k in sorted(set(g) | set(o)) if g.get(k) != o.get(k)}
                print('РАСХОЖДЕНИЕ клиент %s: %s (Go, эталон)' % (g.get('id'), diff))
    extra = sum(1 for _ in go_it) + sum(1 for _ in or_it)
    if extra:
        bad += extra
        print('РАСХОЖДЕНИЕ: число строк разное, лишних %d' % extra)
    print('клиентов: %d, расхождений: %d' % (n, bad))
    return 1 if bad else 0


if __name__ == '__main__':
    sys.exit(main())
