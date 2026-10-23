package source

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
)

// standSeed — ручные аккаунты 1001+ для наглядного демо (SEED_STAND=true).
//
//go:embed seed/stand.json
var standSeed []byte

// seedBody возвращает тело сида: набор организаторов, а при SEED_STAND=true — вместе с аккаунтами стенда.
func seedBody() ([]byte, error) {
	if os.Getenv("SEED_STAND") != "true" {
		return organizers, nil
	}
	return mergeSeeds(organizers, standSeed)
}

// mergeSeeds склеивает два набора: записи второго идут после записей первого.
func mergeSeeds(base, extra []byte) ([]byte, error) {
	var a, b struct {
		Accounts     []json.RawMessage `json:"accounts"`
		Services     []json.RawMessage `json:"services"`
		Transactions []json.RawMessage `json:"transactions"`
	}
	if err := json.Unmarshal(base, &a); err != nil {
		return nil, fmt.Errorf("source: сид: %w", err)
	}
	if err := json.Unmarshal(extra, &b); err != nil {
		return nil, fmt.Errorf("source: сид стенда: %w", err)
	}
	a.Accounts = append(a.Accounts, b.Accounts...)
	a.Services = append(a.Services, b.Services...)
	a.Transactions = append(a.Transactions, b.Transactions...)
	var out bytes.Buffer
	if err := json.NewEncoder(&out).Encode(a); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
