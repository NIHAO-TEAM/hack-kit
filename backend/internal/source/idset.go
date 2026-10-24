package source

// idSetLimit — id ниже этого значения лежат в битовой карте (до 16 МБ на множество), остальные — в map.
const idSetLimit = 1 << 27

// idSet — множество int64 для проверки повторов и ссылок. Идентификаторы снимка — плотные
// положительные числа, поэтому битовая карта занимает бит на id (1 млн платежей — 125 КБ),
// а не десятки байт на запись, как map. Редкие значения вне диапазона уходят в map.
type idSet struct {
	bits  []uint64
	other map[int64]struct{}
}

// add добавляет id и сообщает, был ли он в множестве раньше.
func (s *idSet) add(id int64) (existed bool) {
	if id < 0 || id >= idSetLimit {
		if s.other == nil {
			s.other = map[int64]struct{}{}
		}
		_, existed = s.other[id]
		s.other[id] = struct{}{}
		return existed
	}
	w, b := id>>6, uint64(1)<<(id&63)
	if int(w) >= len(s.bits) {
		grown := make([]uint64, max(int(w)+1, 2*len(s.bits)))
		copy(grown, s.bits)
		s.bits = grown
	}
	existed = s.bits[w]&b != 0
	s.bits[w] |= b
	return existed
}

func (s *idSet) has(id int64) bool {
	if id < 0 || id >= idSetLimit {
		_, ok := s.other[id]
		return ok
	}
	w := id >> 6
	return int(w) < len(s.bits) && s.bits[w]&(uint64(1)<<(id&63)) != 0
}
