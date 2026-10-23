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
