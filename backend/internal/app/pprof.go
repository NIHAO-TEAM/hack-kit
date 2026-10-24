package app

import (
	"net/http"
	"net/http/pprof"
)

// pprofHandler — профили процесса под /debug/pprof/. Маршрута нет в контракте; доступ даёт только
// токен метрик (auth: операция pprof), пользовательская сессия и роли его не открывают.
func pprofHandler() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	m.HandleFunc("/debug/pprof/profile", pprof.Profile)
	m.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	m.HandleFunc("/debug/pprof/trace", pprof.Trace)
	m.HandleFunc("/debug/pprof/", pprof.Index)
	return m
}
