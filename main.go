package main

import (
	"os"

	slurpeethcli "github.com/carlmontanari/slurpeeth/cli"
)

func main() {
	err := slurpeethcli.Entrypoint().Run(os.Args)
	if err != nil {
		panic(err)
	}
}
