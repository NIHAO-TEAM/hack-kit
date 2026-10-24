package source

import (
	"errors"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestStagesObserveAndLint(t *testing.T) {
	var st stages
	_ = st.run(stageParse, func() error { return nil })
	if err := st.run(stageCommit, func() error { return errors.New("x") }); err == nil {
		t.Fatal("ошибка этапа должна вернуться")
	}
	if len(st.names) != 2 {
		t.Fatalf("этапов %d, ждали 2", len(st.names))
	}
	if n := testutil.CollectAndCount(ingestStage, "churn_snapshot_ingest_stage_seconds"); n == 0 {
		t.Fatal("метрика этапов пуста")
	}
	problems, err := testutil.CollectAndLint(ingestStage, "churn_snapshot_ingest_stage_seconds")
	if err != nil || len(problems) > 0 {
		t.Fatalf("promlint: %v %v", err, problems)
	}
}
