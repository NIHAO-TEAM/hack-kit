#!/usr/bin/env bash
# Дописывает в .env недостающие или пустые ключи SSO (те же, что deploy/bootstrap.sh).
# Запуск: deploy/k3s/sso-env.sh <env-файл> <домен>. Существующие непустые значения не трогает.
# Значения генерируются на сервере и не печатаются: в лог идут только имена добавленных ключей.
set -euo pipefail

ENV_FILE="${1:?нужен путь к .env}"
DOMAIN="${2:?нужен домен}"
[ -f "$ENV_FILE" ] || { echo "нет $ENV_FILE" >&2; exit 1; }

ensure() { # ключ, значение
  grep -qE "^$1=.+" "$ENV_FILE" && return 0
  sed -i "/^$1=/d" "$ENV_FILE"
  # Файл без перевода строки в конце: иначе ключ приклеится к последней строке.
  [ -z "$(tail -c1 "$ENV_FILE")" ] || echo >> "$ENV_FILE"
  (umask 077; printf '%s=%s\n' "$1" "$2" >> "$ENV_FILE")
  echo "$1 добавлен в .env"
}

for key in KC_ADMIN_PASSWORD KC_SECURE_PASSWORD KC_SECURE_TOTP_SECRET OIDC_CLIENT_SECRET; do
  grep -qE "^$key=.+" "$ENV_FILE" || ensure "$key" "$(openssl rand -hex 16)"
done
ensure OIDC_CLIENT_ID panel
ensure OIDC_ISSUER_URL "https://$DOMAIN/sso/realms/churn"
ensure OIDC_DISCOVERY_URL "http://keycloak:8080/sso/realms/churn"
ensure OIDC_REDIRECT_URL "https://$DOMAIN/api/v1/auth/oidc/callback"
