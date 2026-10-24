// Команда snapcalc читает снимок и печатает расчёт Go построчно (JSON на клиента):
//
//	go run ./tools/snapcalc/cmd -in snap.json > go.jsonl
package main

import (
	"flag"
	"fmt"
	"os"

	"example.com/app/tools/snapcalc"
)

func main() {
	in := flag.String("in", "", "файл снимка (по умолчанию stdin)")
	asOf := flag.String("as-of", "", "момент расчёта, RFC 3339 (по умолчанию asOf снимка)")
	flag.Parse()
	r := os.Stdin
	if *in != "" {
		f, err := os.Open(*in)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		defer func() { _ = f.Close() }()
		r = f
	}
	if err := snapcalc.Run(r, os.Stdout, *asOf); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
