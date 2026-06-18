package main

import (
	"fmt"
	"os"

	"github.com/pouyasadri/go-blockchain/internal/cli"
)

func main() {
	if err := cli.Execute(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
