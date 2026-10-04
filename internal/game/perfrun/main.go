//go:build perf

// Command perfrun measures the frame times of the game in a real window (see
// game.RunPerf). Run it from the repository root with a display:
//
//	go run -tags perf ./internal/game/perfrun gallery
//	go run -tags perf ./internal/game/perfrun play
//
// It uses a save of its own in a temporary directory.
package main

import (
	"errors"
	"log"
	"os"

	"github.com/nao1215/rabbitrun/internal/game"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("usage: perfrun gallery|play")
	}
	if err := run(os.Args[1]); err != nil {
		log.Fatal(err)
	}
}

// run runs the scenario on a save of its own, removed afterwards.
func run(scenario string) (err error) {
	dir, err := os.MkdirTemp("", "rabbitrun-perf")
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, os.RemoveAll(dir)) }()
	for _, k := range []string{"XDG_CONFIG_HOME", "HOME"} {
		if err := os.Setenv(k, dir); err != nil {
			return err
		}
	}
	if err := game.Load(os.DirFS("assets")); err != nil {
		return err
	}
	return game.RunPerf(scenario, os.Stdout)
}
