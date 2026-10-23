// Package migrations встраивает SQL-миграции goose в бинарь.
package migrations

import "embed"

// FS — файлы миграций; версия — время создания в имени файла.
//
//go:embed *.sql
var FS embed.FS
