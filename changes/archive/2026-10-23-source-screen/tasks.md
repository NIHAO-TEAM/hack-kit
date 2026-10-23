# Задачи: Экран «Источник»: данные, качество, правка услуги, эмулятор

- [x] 1.1 `PatchService`, `PatchAccount` — проверка: тест «баланс №9 +529,84 € → потеря −1 067,48 €»
- [x] 2.1 Экран: шапка, качество, правка, эмулятор, загрузка файла — проверка: скриншоты обеих тем, 1440 и 820
- [x] 2.2 Маркетолог — «недостаточно прав» — проверка: тест роутера (RightGate; сервер — permissions_test и TestPatchAccountRecalculatesLoss; экран — source-screen.test.tsx)
- [x] 9.1 `make check` / `pnpm check` зелёные; сценарии `specs/` проходят (pnpm check зелёный; go-тесты с БД: новые проходят, TestReaderLoadsStandSeed и TestSettingsAPI красные и без этого изменения)
