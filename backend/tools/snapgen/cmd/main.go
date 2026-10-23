// Команда snapgen пишет сгенерированный снимок в stdout:
//
//	go run ./tools/snapgen/cmd -accounts 10000 -seed 1 > snap.json
//	go run ./tools/snapgen/cmd -edge zero-price -accounts 50 | curl -X PUT --data-binary @- …
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"example.com/app/tools/snapgen"
)

func main() {
	var o snapgen.Options
	flag.IntVar(&o.Accounts, "accounts", 100, "число аккаунтов")
	flag.Uint64Var(&o.Seed, "seed", 1, "seed генератора")
	flag.StringVar(&o.Edge, "edge", "", "крайний набор: "+strings.Join(snapgen.Edges, ", "))
	flag.Parse()
	body, err := snapgen.Generate(o)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if _, err := os.Stdout.Write(body); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
