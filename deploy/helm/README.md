# Helm-чарт churn

Чарт `churn/` — прод на Kubernetes (k3s). Выкат на наш сервер — `deploy/k3s/deploy.sh`.

## Встраивание в контур aéza

Встраивание — правка одного файла values. Возьмите `churn/values-aeza.example.yaml`, подставьте свои значения и выполните:

```sh
helm upgrade --install churn deploy/helm/churn -n churn --create-namespace \
  -f values-aeza.yaml --set image.tag=<sha>
```

Что правится в файле:

| Что | Ключи |
|---|---|
| Домен и TLS | `domain`, `ingress.tlsSecret` (чарту нужен Traefik и его CRD `traefik.io`: `ingress.className: traefik`, `networkPolicy.ingressNamespace`) |
| Источник REST | `env.BILLING_URL`, `env.BILLING_POLL_SECONDS` |
| OIDC (внешний Keycloak/IdP) | `env.OIDC_ISSUER_URL`, `env.OIDC_CLIENT_ID`, `env.OIDC_REDIRECT_URL` |
| Prometheus | `serviceMonitor.enabled`, `serviceMonitor.tokenSecret` (нужен prometheus-operator) |
| База | `postgres.enabled: false`, `DATABASE_URL` в Secret |
| Спутники | `companions.enabled: false` |

Секреты заводятся заранее, значения в репозиторий не попадают.

- Secret `churn-aeza` (`existingSecret`): `DATABASE_URL`, `ADMIN_PASSWORD`, `METRICS_TOKEN`, `BILLING_TOKEN`, `OIDC_CLIENT_SECRET`.
- Secret `churn-metrics-token` (`serviceMonitor.tokenSecret`): ключ `token` со значением `METRICS_TOKEN`.
- TLS-секрет `churn-tls` (`ingress.tlsSecret`).

Проверка примера: `helm template churn deploy/helm/churn -f deploy/helm/churn/values-aeza.example.yaml | kubeconform -strict -ignore-missing-schemas` (в CI — `deploy-k8s-values.yml`).
