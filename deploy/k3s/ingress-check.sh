#!/usr/bin/env bash
# 20 HTTPS-запросов подряд к /healthz, каждый на новом соединении, сертификат проверяется.
# Любой не-200 — код 1. Запускается где угодно: PROD_DOMAIN=<домен> deploy/k3s/ingress-check.sh
set -uo pipefail

: "${PROD_DOMAIN:?нужен PROD_DOMAIN}"
N="${N:-20}"
fail=0
for i in $(seq 1 "$N"); do
  code="$(curl -sS -o /dev/null -w '%{http_code}' --max-time 10 "https://$PROD_DOMAIN/healthz" 2>&1)" || true
  echo "$i/$N: $code"
  [ "$code" = "200" ] || fail=$((fail + 1))
done
if [ "$fail" -ne 0 ]; then
  echo "::error::вход нестабилен: $fail из $N запросов без 200" >&2
  exit 1
fi
echo "вход стабилен: $N из $N"
