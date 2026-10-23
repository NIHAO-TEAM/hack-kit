package auth

import "testing"

func TestVerifyPassword(t *testing.T) {
	hash, err := HashPassword("верный-пароль")
	if err != nil {
		t.Fatal(err)
	}
	other, _ := HashPassword("верный-пароль")
	if hash == other {
		t.Error("соль не случайна: два хеша одного пароля совпали")
	}
	tests := []struct {
		name     string
		password string
		encoded  string
		want     bool
	}{
		{"верный пароль", "верный-пароль", hash, true},
		{"неверный пароль", "другой", hash, false},
		{"пустой пароль", "", hash, false},
		{"пустой хеш", "верный-пароль", "", false},
		{"мусор вместо хеша", "верный-пароль", "$argon2id$x$y$z$w", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := VerifyPassword(tt.password, tt.encoded); got != tt.want {
				t.Errorf("VerifyPassword = %v, ждали %v", got, tt.want)
			}
		})
	}
}
