package source

import (
	"errors"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil/promlint"
	"github.com/prometheus/common/expfmt"
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
	reg := prometheus.NewRegistry()
	reg.MustRegister(IngestCollectors()...)
	fams, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	var sb strings.Builder
	for _, f := range fams {
		if _, err := expfmt.MetricFamilyToText(&sb, f); err != nil {
			t.Fatal(err)
		}
	}
	if !strings.Contains(sb.String(), `churn_snapshot_ingest_stage_seconds_count{stage="parse"}`) {
		t.Fatalf("нет этапа parse в метрике:\n%s", sb.String())
	}
	problems, err := promlint.New(strings.NewReader(sb.String())).Lint()
	if err != nil || len(problems) > 0 {
		t.Fatalf("promlint: %v %v", err, problems)
	}
}
