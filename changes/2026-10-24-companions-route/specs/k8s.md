# k8s: изменения сценариев

## ADDED

### Requirement: спутники за единым входом

Прод на k3s SHALL отдавать Grafana по `/grafana` и Keycloak по `/sso` через Traefik.

#### Scenario: Grafana по ссылке

- WHEN без входа открыт `https://<домен>/grafana/`
- THEN перенаправление на `/grafana/login`, форма входа Grafana

#### Scenario: Keycloak discovery

- WHEN запрошен `https://<домен>/sso/realms/churn/.well-known/openid-configuration`
- THEN JSON с `issuer` = `https://<домен>/sso/realms/churn`
