# Заглушка проекта для проверки процесса. В настоящем проекте generate вызывает
# генератор сервера по ../api/openapi.yaml и sqlc (см. backend/AGENTS.md).
GOLANGCI_VERSION := v2.1.6

.PHONY: generate check test

generate:
	@echo "generate: генераторов в заглушке нет"

check:
	test -z "$$(gofmt -l .)"
	go vet ./...
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_VERSION) run ./...
	go test -race ./...

test:
	go test -race -p 1 ./...
