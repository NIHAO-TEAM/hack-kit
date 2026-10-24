package analytics

import "time"

// Overview — сводка последнего пересчёта: из неё берут и ответ /analytics/summary, и метрики
// /metrics, поэтому суммы в них совпадают. Модуль не знает ядра модели: сборка сервера
// переводит результат расчёта в этот вид.
type Overview struct {
	Version           int64
	AsOf              time.Time
	CalculatedAt      time.Time
	Duration          time.Duration
	Stages            Stages
	SettingsVersion   int64
	Levels            map[string]int // high, medium, low, unscored, lost
	ExpectedLossCents int64
	AtRiskCents       int64
	LostCents         int64
	QualityCounts     map[string]int
	LossByFeature     []FeatureLoss // по убыванию потери; сумма равна ExpectedLossCents
	// RecalcCustomers — сколько клиентов пересчитал последний прогон; Partial — только затронутые.
	RecalcCustomers int
	Partial         bool
}

// FeatureLoss — часть ожидаемой потери, отнесённая к одной причине (признаку модели).
type FeatureLoss struct {
	ID        string
	LossCents int64
	Customers int
}

// OverviewFunc возвращает последний результат; nil, пока ни один расчёт не удался.
type OverviewFunc func() *Overview

// Имена уровней в Overview.Levels; совпадают с ключами сводки ядра модели.
var levelNames = []string{"high", "medium", "low", "unscored", "lost"}
