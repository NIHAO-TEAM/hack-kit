#!/usr/bin/env bash
# Подготовка сервера: k3s, helm, Traefik с ACME (Let's Encrypt). Идемпотентный; секретов не печатает.
# Запускается деплоем (deploy-k8s.yml) на сервере. Нужен sudo без пароля.
set -euo pipefail

: "${PROD_DOMAIN:?нужен человек: не задан PROD_DOMAIN}"
K3S_VERSION="${K3S_VERSION:-v1.31.4+k3s1}"
HELM_VERSION="${HELM_VERSION:-v3.16.4}"
ACME_EMAIL="${ACME_EMAIL:-}"

sudo_cmd() {
  if [ "$(id -u)" -eq 0 ]; then "$@"; else sudo -n "$@"; fi
}
if [ "$(id -u)" -ne 0 ] && ! sudo -n true 2>/dev/null; then
  echo "нужен человек: для k3s нужен sudo без пароля." >&2
  echo "выполнить на сервере под root: echo '$(id -un) ALL=(ALL) NOPASSWD:ALL' > /etc/sudoers.d/$(id -un)" >&2
  exit 1
fi

# 1. k3s. kubeconfig читаем пользователю деплоя; встроенный Traefik остаётся, servicelb отдаёт ему 80/443.
if ! command -v k3s >/dev/null 2>&1; then
  # Порты 80/443 должны быть свободны (Compose-стек остановлен: deploy/k3s/cutover.sh).
  if command -v ss >/dev/null 2>&1 && [ -n "$(ss -ltnH 'sport = :80 or sport = :443' 2>/dev/null)" ]; then
    echo "нужен человек: порт 80 или 443 занят, k3s не получит их для Traefik." >&2
    echo "переключение с Compose делает deploy/k3s/cutover.sh (остановит стек Compose, данные сохранит)" >&2
    exit 1
  fi
  echo "ставлю k3s $K3S_VERSION"
  curl -fsSL https://get.k3s.io | sudo_cmd env INSTALL_K3S_VERSION="$K3S_VERSION" \
    INSTALL_K3S_EXEC="server --write-kubeconfig-mode 644" sh -
fi
export KUBECONFIG=/etc/rancher/k3s/k3s.yaml
for _ in $(seq 1 60); do
  if kubectl get nodes 2>/dev/null | grep -q ' Ready'; then break; fi
  sleep 2
done
kubectl get nodes | grep -q ' Ready' || { echo "k3s не стал Ready за 120 с" >&2; exit 1; }
# CRD Traefik ставит traefik-crd позже ноды: без них helm не создаст Middleware.
for _ in $(seq 1 90); do
  kubectl get crd middlewares.traefik.io >/dev/null 2>&1 && break
  sleep 2
done
# Сразу после создания CRD у него ещё нет .status.conditions, и `kubectl wait` падает с accessor error, а не ждёт.
for _ in $(seq 1 90); do
  [ "$(kubectl get crd middlewares.traefik.io -o jsonpath='{.status.conditions[?(@.type=="Established")].status}' 2>/dev/null)" = "True" ] && break
  sleep 2
done
kubectl get crd middlewares.traefik.io -o jsonpath='{.status.conditions[?(@.type=="Established")].status}' | grep -q True \
  || { echo "CRD middlewares.traefik.io не стал Established за 180 с" >&2; exit 1; }

# 2. helm.
if ! command -v helm >/dev/null 2>&1; then
  echo "ставлю helm $HELM_VERSION"
  tmp="$(mktemp -d)"
  curl -fsSL "https://get.helm.sh/helm-$HELM_VERSION-linux-amd64.tar.gz" | tar xz -C "$tmp"
  sudo_cmd install -m 755 "$tmp/linux-amd64/helm" /usr/local/bin/helm
  rm -rf "$tmp"
fi

# 3. Traefik: точка входа websecure с ACME (tls-alpn), редирект http → https, хранилище сертификатов на томе.
# Не пересоздавать том: лимит Let's Encrypt — 5 сертификатов в неделю на домен.
email_arg=""
[ -n "$ACME_EMAIL" ] && email_arg="      - \"--certificatesresolvers.le.acme.email=$ACME_EMAIL\""
sudo_cmd tee /var/lib/rancher/k3s/server/manifests/traefik-config.yaml >/dev/null <<YAML
apiVersion: helm.cattle.io/v1
kind: HelmChartConfig
metadata:
  name: traefik
  namespace: kube-system
spec:
  valuesContent: |-
    additionalArguments:
      - "--certificatesresolvers.le.acme.tlschallenge=true"
      - "--certificatesresolvers.le.acme.storage=/data/acme.json"
${email_arg}
    persistence:
      enabled: true
    ports:
      web:
        redirections:
          entryPoint:
            to: websecure
            scheme: https
YAML
echo "k3s готов"
