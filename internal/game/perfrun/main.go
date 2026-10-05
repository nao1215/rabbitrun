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
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/nao1215/rabbitrun/internal/game"
	"github.com/nao1215/rabbitrun/internal/save"
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
	// the user's config directory on every system (os.UserConfigDir): XDG_CONFIG_HOME on
	// Linux and the BSDs, HOME on macOS, AppData on Windows, where the player's own save was
	// written over
	for _, k := range []string{"XDG_CONFIG_HOME", "HOME", "AppData"} {
		if err := os.Setenv(k, dir); err != nil {
			return err
		}
	}
	if !strings.HasPrefix(save.Path(), dir+string(os.PathSeparator)) {
		return fmt.Errorf("the save would be %s, outside the run's own directory %s", save.Path(), dir)
	}
	if err := game.Load(os.DirFS("assets")); err != nil {
		return err
	}
	return game.RunPerf(scenario, os.Stdout)
}
