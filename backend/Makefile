# generate пишет код сервера и клиента из ../api/openapi.yaml (oapi-codegen, версия в go.mod).
# sqlc: запросы модулей (queries.sql) лежат в internal/<модуль>/, код — в db/.
GOLANGCI_VERSION := v2.1.6
GOVULNCHECK_VERSION := v1.1.4

.PHONY: generate check test test-int parity backtest

generate:
	cp ../api/openapi.yaml internal/platform/docs/openapi.yaml
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen -config oapi-codegen.yaml ../api/openapi.yaml
	go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate

check:
	test -z "$$(gofmt -l .)"
	go vet ./...
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_VERSION) run ./...
	go test -race ./...
	go run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) ./...

test:
	go test -race -p 1 ./...

test-int:
	go test -race -p 1 -tags integration ./...

# parity сверяет Go с эталоном оракула на сгенерированном снимке (N клиентов, SEED); в CI не входит.
# Быстрый вариант на 10 000: go test -tags parity ./tools/snapcalc/
parity:
	../tools/oracle/parity.sh $(or $(N),10000) $(or $(SEED),1)

# backtest — проверка модели на прокрутке 30 суток: организаторы, 10k и 100k (docs/model-check.md).
# Зерно фиксировано: тот же результат при каждом запуске.
backtest:
	go run ./tools/backtest/cmd -in internal/source/seed/organizers.json -as-of 2026-10-19T15:00:00Z -title "Набор организаторов"
	go run ./tools/backtest/cmd -accounts 10000 -title "Генератор, 10 000 аккаунтов"
	go run ./tools/backtest/cmd -accounts 100000 -title "Генератор, 100 000 аккаунтов"
