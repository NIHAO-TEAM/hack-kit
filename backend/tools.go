//go:build tools

// Пакет tools фиксирует версии генераторов в go.mod.
package tools

import (
	_ "github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen"
)
