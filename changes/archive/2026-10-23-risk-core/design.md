# Дизайн: ядро модели

## Подход

Источник правды по правилам — `docs/model-rules.md` и оракул `tools/oracle/oracle.py`. Код повторяет их буквально; расхождение с оракулом — ошибка в Go.

**Файлы пакета `internal/risk`** (каждое следующее изменение этапа добавляет свои файлы и не правит эти):
- `snapshot.go` — входные типы `Snapshot{AsOf, Accounts, Services, Transactions}` с полями схемы организаторов; деньги `int64` (центы); не зависят от `db` и `gen`.
- `settings.go` — `Settings{Weights map[string]float64, High, Medium float64, HorizonDays int, PriceBasis string}`, `Defaults()` собирает веса из реестра; `Validate()` (веса в [0, 1], `High > Medium`, горизонт 1..365).
- `registry.go` — `type Feature interface { ID() string; DefaultWeight() float64; Eval(ctx ServiceCtx) (f float64, reason string, ok bool) }`; `Register(Feature)` из `init()`; порядок — по `ID` для детерминизма.
- `charges.go` — число списаний в `[asOf, asOf + h)` (правило 3), календарный шаг с ограничением дня концом месяца.
- `score.go` — `Score(s Snapshot, st Settings, asOf time.Time) Result`: контекст аккаунта (баланс, бонус, транзакции `prolong` по услуге), признаки по живым услугам, `P = 1 − Π(1 − w·f)`, скор `100·max P` (2 знака при выводе, не при сравнении с порогом), уровни, `R`, `round(P·R)` один раз на услугу, сводка.
- `feature_pwait.go`, `feature_noauto.go` — два признака по правилу 5.

**`ServiceCtx`** даёт признаку услугу, её аккаунт, её транзакции, `asOf` и общий для аккаунта контекст (нужно/есть для `funds` вычисляется в `score.go` один раз на аккаунт и кладётся в контекст — признак `funds` в `risk-features` только читает).

**Результат:** `Result{AsOf, Customers []Customer, Summary}`; `Customer{ID, State, Score *float64, Level *string, ExpectedLossCents, ExposureCents, LostCents, Services []ServiceResult}`; `ServiceResult{ID, P, RCents, LossCents, Features []FeatureHit{ID, F, Weight, Contribution, Reason}}`. Поле `Actions` в `Customer` объявляется пустым срезом — его наполняет `money-actions`. Так же заранее объявляются `ServiceResult.Quality []string`, `Summary.QualityCounts map[string]int` и точка расширения `RegisterCheck(Check)` с пустым реестром проверок — их наполняет `risk-features`; и `RegisterAction(Action)` с пустым реестром действий — его наполняет `money-actions`. `Score` вызывает оба реестра, даже пустые. Интерфейсы: `Check { ID() string; Eval(ServiceCtx) (msg string, hit bool) }`; `Action { ID() string; Applies(CustomerCtx) bool; Apply(Snapshot, accountID int64) Snapshot; Title(CustomerCtx) string }`, где `CustomerCtx` — аккаунт, его услуги с признаками и «нужно/есть». Эффект действия ядро считает само: `Score` по копии снимка, где изменён только этот клиент, и разница его ожидаемой потери, скор и уровень после (функция `scoreOne` для одного клиента, чтобы не пересчитывать весь снимок).

**`priceBasis = per_month`:** у `hour` `R = price` за 720 часов, а не `price × 720` (правило 4).

## Отброшенные варианты

| Вариант | Почему нет |
|---|---|
| Признаки как `switch` в одной функции | новый признак правит общий файл — нет расширяемости (требование 7) |
| Скор как сумма вкладов | шкала уплывает с каждым признаком (`01_solution.md`) |

## Допущения

- [ПРОВЕРЕНО] Правила и эталонные числа — `docs/model-rules.md`, `testdata/golden/organizers.json`.
- [ПРЕДПОЛОЖЕНО] Порядок признаков в причинах — по убыванию вклада, при равенстве — по `ID`.

## Риски

- Округления: скор сравнивается с порогом до округления до 2 знаков; деньги округляются один раз на услугу — иначе центы разойдутся с оракулом.

## Решения по ходу

- **Порог и округление.** `design.md` («Подход») говорит сравнивать с порогом до округления, а оракул `tools/oracle/oracle.py` и `docs/model-rules.md` (правило 6–7: «скор… округление до 2 знаков», «high — скор ≥ 70») сравнивают округлённый скор. Выбрал оракул: расхождение с ним — ошибка Go, golden должен совпасть до цента. Скор вида 69,996 даёт 70,00 и уровень «высокий».
- **Вклад признака** (`FeatureHit.Contribution`) = вес · f — слагаемое под произведением шумового ИЛИ; по нему сортируются причины.
- **Реестры** — пакетные переменные, заполняемые из `init()`; внутри — обобщённый тип с сортировкой по `ID`. Тесты реестра берут свежий экземпляр, глобальный не засоряют.
- **Неизвестный период оплаты** (не из `hour/week/month/quarter_year/half_year/year`) даёт 0 списаний, а не зависание цикла.
- [ПРОВЕРЕНО] Сверка `pwait` и `noauto` с `testdata/golden/organizers.json` по всем услугам набора организаторов — `TestFeaturesMatchOracleGolden`.
