//go:build parity

package snapcalc_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"example.com/app/internal/risk"
	"example.com/app/tools/snapcalc"
	"example.com/app/tools/snapgen"
)

// Go совпадает с эталоном oracle.py на 10 000 сгенерированных клиентов: состояние, уровень, скор, деньги
// в центах, набор действий с ценой. Нужен python3; в обязательный CI не входит (go test -tags parity).
func TestParity10k(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("нет python3")
	}
	body, err := snapgen.Generate(snapgen.Options{Accounts: 10000, Seed: 1})
	if err != nil {
		t.Fatal(err)
	}
	var snap risk.Snapshot
	if err := json.Unmarshal(body, &snap); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "snap.json")
	if err := os.WriteFile(file, body, 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("python3", "-I", "../../../tools/oracle/oracle.py", file,
		"--as-of", snap.AsOf.Format("2006-01-02T15:04:05Z"), "--per-customer")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	want := bufio.NewScanner(&out)
	want.Buffer(nil, 1<<20)
	got := snapcalc.Lines(snap, snap.AsOf)
	bad := 0
	for i, g := range got {
		if !want.Scan() {
			t.Fatalf("в эталоне строк меньше, чем клиентов Go (%d)", len(got))
		}
		var w snapcalc.Line
		if err := json.Unmarshal(want.Bytes(), &w); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(g, w) {
			if bad++; bad <= 20 {
				t.Errorf("строка %d, клиент %d: Go %+v, эталон %+v", i, g.ID, g, w)
			}
		}
	}
	if want.Scan() {
		t.Error("в эталоне строк больше, чем клиентов Go")
	}
	if bad > 0 {
		t.Fatalf("расхождений: %d из %d", bad, len(got))
	}
}
