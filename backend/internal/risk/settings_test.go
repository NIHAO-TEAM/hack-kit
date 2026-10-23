package risk

import "testing"

func TestSettingsValidate(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*Settings)
		bad  bool
	}{
		{"умолчания", func(*Settings) {}, false},
		{"вес 1,2", func(s *Settings) { s.Weights["pwait"] = 1.2 }, true},
		{"вес -0,1", func(s *Settings) { s.Weights["pwait"] = -0.1 }, true},
		{"high равен medium", func(s *Settings) { s.High = s.Medium }, true},
		{"high меньше medium", func(s *Settings) { s.High = 40 }, true},
		{"горизонт 0", func(s *Settings) { s.HorizonDays = 0 }, true},
		{"горизонт 366", func(s *Settings) { s.HorizonDays = 366 }, true},
		{"горизонт 365", func(s *Settings) { s.HorizonDays = 365 }, false},
		{"priceBasis", func(s *Settings) { s.PriceBasis = "x" }, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := Defaults()
			c.mut(&s)
			if err := s.Validate(); (err != nil) != c.bad {
				t.Fatalf("Validate() = %v, ждали ошибку: %v", err, c.bad)
			}
		})
	}
}

func TestDefaults(t *testing.T) {
	s := Defaults()
	if s.High != 70 || s.Medium != 50 || s.HorizonDays != 30 || s.PriceBasis != PricePerPeriod {
		t.Fatalf("умолчания: %+v", s)
	}
	if s.Weights["pwait"] != 0.80 || s.Weights["noauto"] != 0.55 {
		t.Fatalf("веса: %v", s.Weights)
	}
}
