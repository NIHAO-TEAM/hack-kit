# sso: изменения сценариев

## ADDED

### Requirement: Keycloak на проде

Прод SHALL отдавать discovery Keycloak по `/sso` после каждого выката.

#### Scenario: discovery

- WHEN запрошен `https://<домен>/sso/realms/churn/.well-known/openid-configuration`
- THEN 200, JSON с `issuer` = `https://<домен>/sso/realms/churn`
