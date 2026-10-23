# generate пишет код сервера и клиента из ../api/openapi.yaml (oapi-codegen, версия в go.mod).
# sqlc: запросы модулей (queries.sql) лежат в internal/<модуль>/, код — в db/.
GOLANGCI_VERSION := v2.1.6
GOVULNCHECK_VERSION := v1.1.4

.PHONY: generate check test test-int

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
