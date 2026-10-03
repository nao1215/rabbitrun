package main

import (
	"fmt"
	"io"
	"log"
	"os"

	flag "github.com/spf13/pflag"
)

// printUsage prints a short help to stderr (used when an option is wrong).
func printUsage() { writeUsage(os.Stderr) }

// writeUsage writes a short help: what the game is, its options, and where to go next.
func writeUsage(w io.Writer) {
	if _, err := fmt.Fprintf(w, `Rabbit Run - a sweets-themed road runner

Usage:
  rabbitrun [options]

Options:
%s
Homepage:        https://github.com/nao1215/rabbitrun
Report an issue: https://github.com/nao1215/rabbitrun/issues
GitHub Sponsors: https://github.com/sponsors/nao1215
`, flag.CommandLine.FlagUsages()); err != nil {
		log.Print(err)
	}
}
