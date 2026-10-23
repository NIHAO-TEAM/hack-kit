package auth_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"example.com/app/internal/apitest"
)

// TestJuryPath — путь жюри из кейса: вход по паролю, затем изменение настроек, загрузка
// снимка и чтение сводки проходят авторизацию (тела пока 501).
func TestJuryPath(t *testing.T) {
	s := apitest.New(t)
	ctx := context.Background()
	jury := s.As(t, "jury")

	patch, err := jury.PatchModelSettingsWithBody(ctx, nil, "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	_ = patch.Body.Close()
	snapshot, err := jury.PutSnapshotWithBody(ctx, "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	_ = snapshot.Body.Close()
	summary, err := jury.GetSummaryWithResponse(ctx)
	if err != nil {
		t.Fatal(err)
	}

	for name, code := range map[string]int{
		"PATCH /model/settings": patch.StatusCode, "PUT /source/snapshot": snapshot.StatusCode, "GET /analytics/summary": summary.StatusCode(),
	} {
		if code == http.StatusUnauthorized || code == http.StatusForbidden {
			t.Errorf("%s: %d", name, code)
		}
	}
}
