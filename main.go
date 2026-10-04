// Command rabbitrun is Rabbit Run, a short, sweets-themed road runner: steer a bunny up a
// road of gummy blocks with a character who reacts to your run.
//
// This package only reads the command line and opens the window; the game itself is in
// internal/game.
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/hajimehoshi/ebiten/v2"
	flag "github.com/spf13/pflag"

	"github.com/nao1215/rabbitrun/internal/game"
	"github.com/nao1215/rabbitrun/internal/save"
	"github.com/nao1215/rabbitrun/internal/sound"
)

func main() {
	// Mistakes are reported by exitUsage: the error first, then where the help is.
	flag.CommandLine.Init("rabbitrun", flag.ContinueOnError)
	if err := flag.CommandLine.Parse(os.Args[1:]); err != nil {
		exitUsage(err)
	}
	if *showHelp {
		writeUsage(os.Stdout)
		return
	}
	if *showVersion {
		if _, err := fmt.Println("rabbitrun " + version); err != nil {
			log.Fatal(err)
		}
		return
	}
	if err := checkArgs(flag.CommandLine); err != nil {
		exitUsage(err)
	}
	for _, dir := range []string{*bgmWavDir, *captureDir} {
		if dir == "" {
			continue
		}
		if err := os.MkdirAll(dir, 0o750); err != nil {
			log.Fatal(err)
		}
	}
	if *bgmWavDir != "" {
		if err := sound.WriteBGMWavs(*bgmWavDir); err != nil {
			log.Fatal(err)
		}
		return
	}
	if *resetSaveFlag {
		// only resets the save data; the game is started again without the flag
		moved, err := save.Reset()
		if err != nil {
			log.Fatal(err)
		}
		msg := fmt.Sprintf("the save data was reset (the old one is kept as %s.bak)", save.Path())
		if !moved {
			msg = fmt.Sprintf("there is no save data to reset (looked for %s)", save.Path())
		}
		if _, err := fmt.Println(msg); err != nil {
			log.Fatal(err)
		}
		return
	}
	if err := game.Load(embeddedAssets()); err != nil {
		log.Fatal(err)
	}
	if *recordPath != "" {
		// told before a window opens, rather than after the game has loaded
		if err := checkRecordChar(*recordChar, game.CharacterIDs()); err != nil {
			exitUsage(err)
		}
		if err := findFFmpeg(); err != nil {
			log.Fatal(err)
		}
	}
	g, err := game.New(game.Options{
		CaptureDir:    *captureDir,
		Debug:         *debugFlag,
		RecordPath:    *recordPath,
		RecordChar:    *recordChar,
		RecordStage:   *recordStage,
		RecordSeconds: *recordSeconds,
		RecordExtra:   *recordExtra,
	})
	if err != nil {
		log.Fatal(err)
	}

	ebiten.SetWindowTitle("Rabbit Run")
	// Shrink the window so it fits the monitor height.
	w, h := game.ScreenW, game.ScreenH
	if m := ebiten.Monitor(); m != nil {
		if _, mh := m.Size(); mh > 0 && mh*85/100 < h {
			h = mh * 85 / 100
			w = h * game.ScreenW / game.ScreenH
		}
	}
	ebiten.SetWindowSize(w, h)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	err = ebiten.RunGame(g)
	g.Close() // the window was closed: whatever changed last is written
	if err != nil {
		log.Fatal(err)
	}
}
