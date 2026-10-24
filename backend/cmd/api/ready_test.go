package main

import (
	"errors"
	"fmt"
	"testing"

	"example.com/app/internal/source"
)

func TestReadyOnEmptyDatabase(t *testing.T) {
	cases := []struct {
		name      string
		hasResult bool
		lastErr   error
		want      bool
	}{
		{"есть результат", true, nil, true},
		{"первого прогона ещё не было", false, nil, false},
		{"снимок не загружен — считать нечего", false, fmt.Errorf("чтение: %w", source.ErrNoSnapshot), true},
		{"другая ошибка пересчёта", false, errors.New("база недоступна"), false},
	}
	for _, c := range cases {
		if got := ready(c.hasResult, c.lastErr); got != c.want {
			t.Errorf("%s: ready = %v, ждём %v", c.name, got, c.want)
		}
	}
}
