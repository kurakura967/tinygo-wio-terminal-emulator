package main

import (
	"fmt"
	"os"

	"github.com/kurakura967/tinygo-wio-terminal-emulator/runner"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: wio-emu <path/to/main.go>")
		os.Exit(1)
	}
	if err := runner.Run(os.Args[1]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
