#!/usr/bin/env bash
# Диагностика входа на 80/443: кто слушает порты, какие контейнеры и сервисы k3s их держат.
# Только чтение, секретов не печатает. Запускается на сервере: deploy/k3s/diagnose.sh
set -uo pipefail

export KUBECONFIG="${KUBECONFIG:-/etc/rancher/k3s/k3s.yaml}"
sudo_cmd() { if [ "$(id -u)" -eq 0 ]; then "$@"; else sudo -n "$@"; fi; }

echo "== слушатели 80/443 =="
sudo_cmd ss -ltnp 'sport = :80 or sport = :443' 2>/dev/null || ss -ltn 'sport = :80 or sport = :443' || true
echo "== docker ps =="
docker ps --format 'table {{.Names}}\t{{.Image}}\t{{.Ports}}\t{{.Status}}' 2>&1 || true
echo "== k3s: svc, ingress =="
kubectl get svc,ingress -A -o wide 2>&1 || true
echo "== спутники: endpoints и endpointslices =="
kubectl -n churn get endpoints,endpointslices -o wide 2>&1 || true
echo "== Traefik: версия и предупреждения про спутников =="
kubectl -n kube-system get deploy traefik -o jsonpath='{.spec.template.spec.containers[0].image}{"\n"}' 2>&1 || true
kubectl -n kube-system logs deploy/traefik --tail=500 2>&1 | grep -i -E 'churn-grafana|churn-keycloak|endpoint' | tail -n 20 || true
echo "== спутники: порты на хосте =="
ss -ltn 2>/dev/null | grep -E ':(18080|13000|19090)\b' || echo "порты спутников не слушаются"
echo "== наличие ключей в Secret релиза (значения не печатаются) =="
for k in DEMO_JURY_PASSWORD METRICS_TOKEN OIDC_CLIENT_SECRET; do
  if kubectl -n churn get secret churn-app -o jsonpath="{.data.$k}" 2>/dev/null | grep -q .; then echo "$k: есть"; else echo "$k: нет"; fi
done
echo "== Prometheus: targets =="
bind="$(grep -E '^COMPANIONS_BIND=' "$HOME/churn/.env" 2>/dev/null | cut -d= -f2-)"
curl -fsS --max-time 5 "http://${bind:-172.17.0.1}:19090/api/v1/targets" 2>/dev/null \
  | grep -o '"health":"[a-z]*"' | sort | uniq -c || echo "Prometheus не отвечает"
