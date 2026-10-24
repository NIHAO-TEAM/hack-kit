package source

import "testing"

func TestIDSet(t *testing.T) {
	var s idSet
	for _, id := range []int64{0, 1, 63, 64, 1_200_000, idSetLimit - 1, idSetLimit, -5, 1 << 40} {
		if s.has(id) {
			t.Fatalf("id %d есть в пустом множестве", id)
		}
		if s.add(id) {
			t.Fatalf("первое добавление %d сообщило повтор", id)
		}
		if !s.has(id) || !s.add(id) {
			t.Fatalf("id %d не найден после добавления", id)
		}
	}
	if s.has(2) || s.has(-6) {
		t.Fatal("чужой id найден")
	}
}
