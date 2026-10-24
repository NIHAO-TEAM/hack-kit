//go:build integration

package app_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"example.com/app/internal/apitest"
	"example.com/app/internal/auth"
)

// Загрузка снимка оставляет в /metrics все этапы replace и parse/validate, а вторая загрузка
// во время пересчёта принимается, и сводка в итоге строится по её версии.
func TestSnapshotIngestStagesAndSecondLoad(t *testing.T) {
	body, err := os.ReadFile("../source/seed/organizers.json")
	if err != nil {
		t.Fatal(err)
	}
	r := newRig(t, false)
	r.StartRecalc(t)
	ctx := context.Background()
	loader := r.WithBearer(t, apitest.SourceToken)

	var last int64
	for i := 0; i < 2; i++ {
		res, err := loader.PutSnapshotWithBodyWithResponse(ctx, "application/json", bytes.NewReader(body))
		if err != nil || res.JSON200 == nil {
			t.Fatalf("PUT %d: %v %d %s", i+1, err, res.StatusCode(), res.Body)
		}
		last = res.JSON200.Version
	}

	marketer := r.As(t, auth.RoleMarketer)
	deadline := time.Now().Add(20 * time.Second)
	for {
		sum, err := marketer.GetSummaryWithResponse(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if sum.JSON200 != nil && sum.JSON200.Version == last {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("сводка не дошла до версии второго снимка %d", last)
		}
		time.Sleep(20 * time.Millisecond)
	}

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, r.URL+"/metrics", nil)
	req.Header.Set("Authorization", "Bearer "+apitest.MetricsToken)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	for _, stage := range []string{"parse", "validate", "lock", "truncate", "copy_accounts", "copy_services", "copy_transactions", "sequences", "commit"} {
		if !strings.Contains(string(raw), `churn_snapshot_ingest_stage_seconds_count{stage="`+stage+`"}`) {
			t.Errorf("в /metrics нет этапа %s", stage)
		}
	}
}
