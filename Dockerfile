# syntax=docker/dockerfile:1
# Контекст сборки — весь репозиторий: docker build .

# 1. Фронт: собираем, если в frontend/package.json есть скрипт build; иначе пустой dist/.
FROM node:22-alpine AS front
RUN corepack enable
WORKDIR /src/frontend
COPY frontend/ ./
RUN mkdir -p dist \
 && if node -e 'process.exit(require("./package.json").scripts?.build ? 0 : 1)'; then \
      pnpm install --frozen-lockfile && pnpm build; \
    fi

# 2. Go-бинарь со встроенным фронтом. Версия Go — как в backend/go.mod.
FROM golang:1.24.7-alpine AS build
WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
# Собранный фронт заменяет заглушку; пустой dist/ оставляет её.
COPY --from=front /src/frontend/dist/ /tmp/front-dist/
RUN if [ -n "$(ls -A /tmp/front-dist)" ]; then \
      rm -rf internal/platform/webui/dist && cp -r /tmp/front-dist internal/platform/webui/dist; \
    fi
# PGO: профиль backend/cmd/api/default.pgo; флаг явный, чтобы было видно. Обновление профиля — README.
RUN CGO_ENABLED=0 go build -pgo=auto -trimpath -ldflags="-s -w" -o /out/api ./cmd/api \
 && go version -m /out/api | grep -- "-pgo="

# 3. Итоговый образ: без shell, не от root.
FROM gcr.io/distroless/static:nonroot
COPY --from=build /out/api /api
USER nonroot
EXPOSE 8080
ENTRYPOINT ["/api"]
