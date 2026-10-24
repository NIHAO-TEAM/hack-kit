package plural

import "testing"

func TestForm(t *testing.T) {
	cases := map[int]string{0: "many", 1: "one", 2: "few", 5: "many", 11: "many", 12: "many", 21: "one", 22: "few", 25: "many", 111: "many"}
	for n, want := range cases {
		if got := Form(n, "one", "few", "many"); got != want {
			t.Errorf("Form(%d) = %q, want %q", n, got, want)
		}
	}
}
