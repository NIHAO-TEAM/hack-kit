# generate пишет код сервера и клиента из ../api/openapi.yaml (oapi-codegen, версия в go.mod).
# sqlc подключат модули, у которых появятся queries.sql.
GOLANGCI_VERSION := v2.1.6

.PHONY: generate check test test-int

generate:
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen -config oapi-codegen.yaml ../api/openapi.yaml

check:
	test -z "$$(gofmt -l .)"
	go vet ./...
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_VERSION) run ./...
	go test -race ./...

test:
	go test -race -p 1 ./...

test-int:
	go test -race -p 1 -tags integration ./...
