package source

import (
	"reflect"
	"testing"
	"time"
)

func seedForStream(t *testing.T) Snapshot {
	t.Helper()
	s, err := resetSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	s.AsOf = &seedAsOf
	return *s
}

func TestStreamIsDeterministic(t *testing.T) {
	s := seedForStream(t)
	a := GenerateStream(s, 500, 7)
	b := GenerateStream(s, 500, 7)
	if len(a) != 500 {
		t.Fatalf("событий %d, ждали 500", len(a))
	}
	if !reflect.DeepEqual(a, b) {
		t.Error("то же зерно дало разные последовательности")
	}
	if reflect.DeepEqual(a, GenerateStream(s, 500, 8)) {
		t.Error("разные зёрна дали одну последовательность")
	}
}

// Каждое событие применимо к снимку после предыдущих, а снимок остаётся допустимым.
func TestStreamEventsAreApplicableInOrder(t *testing.T) {
	s := seedForStream(t)
	evs := GenerateStream(s, 2500, 1)
	kinds := map[string]int{}
	for i, ev := range evs {
		if err := applyStreamEvent(&s, ev); err != nil {
			t.Fatalf("событие %d (%s) не применилось: %v", i, ev.Kind, err)
		}
		kinds[ev.Kind]++
	}
	for _, k := range []string{EventTopUp, EventAutoprolongOn, EventAutoprolongOff, EventServiceStop, EventServiceResume, EventRenewalFailed, EventProlongWait} {
		if kinds[k] == 0 {
			t.Errorf("в смеси нет событий %s: %v", k, kinds)
		}
	}
	if share := kinds[EventTopUp] * 100 / len(evs); share < 30 || share > 50 {
		t.Errorf("доля пополнений %d %%, ждали около 40", share)
	}
}

func TestStreamOnEmptySnapshot(t *testing.T) {
	if got := GenerateStream(Snapshot{}, 100, 1); got != nil {
		t.Errorf("пустой снимок дал %d событий", len(got))
	}
}

// Услуг нет: остаются только пополнения.
func TestStreamWithoutServicesFallsBackToTopUps(t *testing.T) {
	s := Snapshot{Accounts: []Account{{ID: 1, Email: "a@x.test", BonusState: "locked"}}}
	for _, ev := range GenerateStream(s, 100, 3) {
		if ev.Kind != EventTopUp {
			t.Fatalf("без услуг событие %s", ev.Kind)
		}
	}
}

func TestPercentile(t *testing.T) {
	d := []time.Duration{}
	for i := 1; i <= 100; i++ {
		d = append(d, time.Duration(i)*time.Millisecond)
	}
	for p, want := range map[int]time.Duration{50: 50 * time.Millisecond, 95: 95 * time.Millisecond, 100: 100 * time.Millisecond} {
		if got := percentile(d, p); got != want {
			t.Errorf("p%d = %v, ждали %v", p, got, want)
		}
	}
	if percentile(nil, 95) != 0 {
		t.Error("у пустого набора процентиль 0")
	}
}
