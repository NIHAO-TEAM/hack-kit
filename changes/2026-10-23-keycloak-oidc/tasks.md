# Задачи: Вход через OIDC (Keycloak с ключом доступа и TOTP) второй кнопкой

- [x] 1.1 OIDC login/callback — проверка: интеграционный тест с Keycloak в testcontainers или с фейковым IdP
- [ ] 1.2 Realm JSON (WebAuthn, TOTP) и профиль `sso` — проверка: `docker compose --profile sso up`, вход `secure`
- [ ] 1.3 Прод `/sso`, админка закрыта — проверка: `/sso/admin` снаружи → 403/404
- [ ] 9.1 `make check` / `pnpm check` зелёные; сценарии `specs/` проходят
