package analytics

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"example.com/app/internal/platform/api/gen"
)

// cursor — место в списке: версия пересчёта, сортировка и позиция последней отданной строки в порядке
// модели чтения. Строка остаётся на своём месте, пока версия та же, поэтому обход курсором не
// повторяет и не пропускает клиентов; после нового пересчёта курсор отвергается (409).
type cursor struct {
	Version int64
	Sort    gen.ListCustomersParamsSort
	Desc    bool
	Pos     int
}

func encodeCursor(c cursor) string {
	d := "a"
	if c.Desc {
		d = "d"
	}
	raw := fmt.Sprintf("%d.%s.%s.%d", c.Version, c.Sort, d, c.Pos)
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeCursor(s string) (cursor, error) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return cursor{}, err
	}
	parts := strings.Split(string(raw), ".")
	if len(parts) != 4 || (parts[2] != "a" && parts[2] != "d") {
		return cursor{}, errors.New("неверный курсор")
	}
	v, err1 := strconv.ParseInt(parts[0], 10, 64)
	pos, err2 := strconv.Atoi(parts[3])
	if err1 != nil || err2 != nil || pos < 0 || !validSort(gen.ListCustomersParamsSort(parts[1])) {
		return cursor{}, errors.New("неверный курсор")
	}
	return cursor{Version: v, Sort: gen.ListCustomersParamsSort(parts[1]), Desc: parts[2] == "d", Pos: pos}, nil
}

func validSort(s gen.ListCustomersParamsSort) bool {
	for _, k := range sortKeys {
		if k == s {
			return true
		}
	}
	return false
}

// matches — выдан ли курсор для той же сортировки и порядка, что в запросе (по умолчанию — потеря, по убыванию).
func (c cursor) matches(sortBy *gen.ListCustomersParamsSort, order *gen.ListCustomersParamsOrder) bool {
	s, desc := gen.ListCustomersParamsSort("expectedLoss"), true
	if sortBy != nil {
		s = *sortBy
	}
	if order != nil {
		desc = *order != gen.Asc
	}
	return c.Sort == s && c.Desc == desc
}
