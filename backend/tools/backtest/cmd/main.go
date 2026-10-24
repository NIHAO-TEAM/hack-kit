// Команда backtest прокручивает снимок на 30 суток и печатает markdown-отчёт:
//
//	go run ./tools/backtest/cmd -in snap.json
//	go run ./tools/backtest/cmd -accounts 10000 -seed 1
//
// Без -in и -accounts берётся снимок организаторов из сида сервера.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"

	"example.com/app/internal/source"
	"example.com/app/tools/backtest"
	"example.com/app/tools/snapgen"
)

func main() {
	in := flag.String("in", "", "файл снимка")
	accounts := flag.Int("accounts", 0, "сгенерировать снимок на N аккаунтов (snapgen)")
	genSeed := flag.Uint64("gen-seed", 1, "зерно генератора снимка")
	seed := flag.Uint64("seed", 1, "зерно поведения клиентов")
	behaviorFile := flag.String("behavior", "", "файл допущений поведения (по умолчанию вшитый behavior.json)")
	title := flag.String("title", "", "заголовок раздела")
	robust := flag.Bool("robust", true, "прогнать устойчивость ±50 %")
	flag.Parse()
	if err := run(*in, *accounts, *genSeed, *seed, *behaviorFile, *title, *robust); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(in string, accounts int, genSeed, seed uint64, behaviorFile, title string, robust bool) error {
	var body []byte
	var err error
	switch {
	case accounts > 0:
		body, err = snapgen.Generate(snapgen.Options{Accounts: accounts, Seed: genSeed})
	case in != "":
		body, err = os.ReadFile(in)
	default:
		return fmt.Errorf("укажите -in файл или -accounts N")
	}
	if err != nil {
		return err
	}
	snap, ferrs, err := source.ParseSnapshot(bytes.NewReader(body))
	if err != nil || len(ferrs) > 0 {
		return fmt.Errorf("разбор снимка: %v %v", err, ferrs)
	}
	b, err := backtest.DefaultBehavior()
	if behaviorFile != "" {
		b, err = backtest.LoadBehavior(behaviorFile)
	}
	if err != nil {
		return err
	}
	p, err := backtest.Prepare(*snap)
	if err != nil {
		return err
	}
	if title == "" {
		title = "Снимок"
	}
	backtest.Section(os.Stdout, title)
	fmt.Fprintf(os.Stdout, "Зерно поведения: %d.\n\n", seed)
	backtest.WriteResult(os.Stdout, "Основной прогон", p.Run(b, seed))
	if robust {
		fmt.Fprintf(os.Stdout, "### Устойчивость: вероятности ×0,5 и ×1,5\n\n")
		backtest.WriteRobustness(os.Stdout, p.Robustness(b, seed))
	}
	return nil
}
