// Package risk — признаки, скор, уровни и деньги. HTTP-путей у модуля нет: им пользуются
// analytics и model. Пакет — чистые функции: без ввода-вывода, без time.Now() (время приходит
// аргументом asOf), без импорта db, gen и net/http. Правила — docs/model-rules.md.
//
// Score(snapshot, settings, asOf) считает вероятность непродления каждой живой услуги шумовым
// ИЛИ P = 1 − Π(1 − w·f), скор клиента 100·max P, уровень по порогам, деньги услуги R,
// ожидаемую потерю round(P·R), «под угрозой» и «уже потеряно».
//
// # Как добавить признак
//
// Один файл feature_<id>.go в этом пакете, больше ничего править не нужно:
//
//	type myFeature struct{}
//
//	func init() { Register(myFeature{}) }
//
//	func (myFeature) ID() string             { return "myid" }
//	func (myFeature) DefaultWeight() float64 { return 0.3 }
//	func (myFeature) Eval(ctx ServiceCtx) (f float64, reason string, ok bool) { ... }
//
// Eval возвращает f ∈ [0, 1], текст причины простыми словами и ok = «признак применим к услуге».
// Вес признака появляется в Defaults().Weights, вклад — в ServiceResult.Features.
// Аналогично добавляются проверки качества данных (RegisterCheck) и действия с ценой (RegisterAction).
// Результаты реестров не зависят от порядка init(): элементы сортируются по ID.
package risk
