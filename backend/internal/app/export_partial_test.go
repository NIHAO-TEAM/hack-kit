package app

import (
	"example.com/app/internal/analytics"
	"example.com/app/internal/risk"
)

// BuildPortfolioForTest отдаёт тестам сборку портфеля с нуля.
func BuildPortfolioForTest(res risk.Result, snap risk.Snapshot, version int64) *analytics.Portfolio {
	return buildPortfolio(res, snap, version)
}
