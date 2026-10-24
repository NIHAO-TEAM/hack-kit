#!/usr/bin/env bash
# Выкат чарта на k3s. Запускается на сервере: SHA=<sha> PROD_DOMAIN=<домен> deploy/k3s/deploy.sh
# Образ собирается здесь и импортируется в containerd k3s. Не прошла проверка /healthz — helm rollback.
# Секреты берутся из ~/churn/.env (создаёт deploy/bootstrap.sh), во временный values-файл (600), нигде не печатаются.
set -euo pipefail

: "${SHA:?нужен SHA}"
: "${PROD_DOMAIN:?нужен PROD_DOMAIN}"
APP_DIR="${APP_DIR:-$HOME/churn}"
ENV_FILE="$APP_DIR/.env"
SRC="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
RELEASE="${RELEASE:-churn}"
NS="${NS:-churn}"
export KUBECONFIG="${KUBECONFIG:-/etc/rancher/k3s/k3s.yaml}"
EXTRA=("$@")

[ -f "$ENV_FILE" ] || { echo "нет $ENV_FILE: сначала deploy/bootstrap.sh" >&2; exit 1; }

sudo_cmd() { if [ "$(id -u)" -eq 0 ]; then "$@"; else sudo -n "$@"; fi; }

echo "собираю образ churn-app:$SHA"
docker build -q -t "churn-app:$SHA" "$SRC" >/dev/null
docker save "churn-app:$SHA" | sudo_cmd k3s ctr images import - >/dev/null

# Ключи SSO дописываются до values: приложение и Keycloak получают один OIDC_CLIENT_SECRET в одном выкате.
"$(dirname "${BASH_SOURCE[0]}")/sso-env.sh" "$ENV_FILE" "$PROD_DOMAIN"

values="$(mktemp)"; trap 'rm -f "$values"' EXIT
chmod 600 "$values"
{
  echo "domain: \"$PROD_DOMAIN\""
  echo "postgres:"
  echo "  password: \"$(grep -E '^POSTGRES_PASSWORD=' "$ENV_FILE" | cut -d= -f2-)\""
  echo "secretEnv:"
  # Всё, кроме POSTGRES_PASSWORD, KC_* (ключи Keycloak, приложению не нужны) и уже заданного в env чарта, идёт в Secret приложения.
  grep -E '^[A-Z_]+_(PASSWORD|TOKEN)=|^ADMIN_LOGIN=|^OIDC_CLIENT_SECRET=' "$ENV_FILE" | grep -vE '^(POSTGRES_PASSWORD|KC_[A-Z_]*)=' \
    | awk -F= '{ k=$1; sub(/^[^=]*=/, ""); printf "  %s: \"%s\"\n", k, $0 }'
  echo "env:"
  # OIDC: discovery читается из Keycloak по Service чарта (спутник за пределами k3s), а не по имени Compose.
  grep -E '^(SEED_ORGANIZERS|COOKIE_SECURE|TRUSTED_PROXY|OIDC_ISSUER_URL|OIDC_DISCOVERY_URL|OIDC_CLIENT_ID|OIDC_REDIRECT_URL)=' "$ENV_FILE" \
    | sed "s|http://keycloak:8080|http://$RELEASE-keycloak:8080|" | awk -F= '{ k=$1; sub(/^[^=]*=/, ""); printf "  %s: \"%s\"\n", k, $0 }'
} > "$values"

# Узел должен вмещать лимиты памяти чарта: иначе под зависнет в Pending или узел уйдёт в OOM.
# Не вмещает — выкат красный, лимиты и прод не трогаем (нужен человек).
alloc_ki="$(kubectl get nodes -o jsonpath='{.items[0].status.allocatable.memory}' 2>/dev/null | sed 's/Ki$//')"
need_mi="$(helm template "$RELEASE" "$SRC/deploy/helm/churn" -n "$NS" --set domain=x --set postgres.password=x 2>/dev/null \
  | awk '/^ *limits:/ {l=1; next} l && /memory:/ {v=$2; gsub(/"/, "", v); if (v ~ /Gi$/) {sub(/Gi$/, "", v); n+=v*1024} else if (v ~ /Mi$/) {sub(/Mi$/, "", v); n+=v}} {if ($0 !~ /memory:/ && $0 !~ /^ *limits:/) l=0} END {print n+0}')"
if [[ "$alloc_ki" =~ ^[0-9]+$ ]]; then
  echo "узел: allocatable память $((alloc_ki / 1024)) МиБ, лимиты чарта (без спутников Compose) $need_mi МиБ"
  if [ "$need_mi" -gt "$((alloc_ki / 1024))" ]; then
    echo "::error::нужен человек: лимиты памяти чарта ($need_mi МиБ) больше allocatable узла ($((alloc_ki / 1024)) МиБ); выкат не начат"
    exit 1
  fi
else
  echo "узел: allocatable память не прочитана, проверка пропущена"
fi

prev_rev="$(helm -n "$NS" history "$RELEASE" -o json 2>/dev/null | grep -o '"revision":[0-9]*' | tail -1 | cut -d: -f2 || true)"

wait_healthy() {
  local limit="$1" i=0
  while [ "$i" -lt "$limit" ]; do
    if curl -fsS --max-time 5 "https://$PROD_DOMAIN/healthz" >/dev/null 2>&1; then return 0; fi
    sleep 5; i=$((i + 5))
  done
  return 1
}

limit=120; [ -z "$prev_rev" ] && limit=300
# --atomic откатывает релиз сам, если pod не стал Ready; проверка снаружи ловит остальное (TLS, Ingress).
if helm upgrade --install "$RELEASE" "$SRC/deploy/helm/churn" -n "$NS" --create-namespace \
     -f "$values" --set image.tag="$SHA" --atomic --timeout 8m "${EXTRA[@]}" \
   && { [ -n "${SKIP_HEALTH:-}" ] || wait_healthy "$limit"; }; then
  echo "выкачено: $SHA"
  exit 0
fi

echo "::error::выкат $SHA не прошёл: helm упал или /healthz не отвечает"
if [ -n "$prev_rev" ]; then
  echo "откат: helm rollback $RELEASE $prev_rev"
  helm rollback "$RELEASE" "$prev_rev" -n "$NS" --wait --timeout 5m || true
fi
exit 1
