package main

import (
	"fmt"
	"log"
	"os"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	flag "github.com/spf13/pflag"

	"github.com/nao1215/rabbitrun/internal/save"
	"github.com/nao1215/rabbitrun/internal/sound"
)

// Portrait screen (4:5).
const (
	ScreenW = 720
	ScreenH = 900
)

type Scene interface {
	Update(g *Game)
	Draw(screen *ebiten.Image)
}

type Game struct {
	in    Input
	scene Scene
	frame int
	bg    *background
	cap   *captureState
	rec   *recorder
}

// SetScene switches to the scene s. The scene left frees what it holds on the GPU, if it
// has a release method.
func (g *Game) SetScene(s Scene) {
	if r, ok := g.scene.(interface{ release() }); ok && g.scene != s {
		r.release()
	}
	store.Flush()
	g.scene = s
}

func (g *Game) Update() error {
	if g.rec != nil && g.rec.skipUpdate() {
		return nil
	}
	g.frame++
	g.in.Update()
	if inpututil.IsKeyJustPressed(ebiten.KeyF11) ||
		(ebiten.IsKeyPressed(ebiten.KeyAlt) && inpututil.IsKeyJustPressed(ebiten.KeyEnter)) {
		ebiten.SetFullscreen(!ebiten.IsFullscreen())
	}
	g.bg.update()
	if g.cap != nil {
		g.cap.update(g)
	}
	g.scene.Update(g)
	store.Flush()
	if quitRequested {
		return ebiten.Termination
	}
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	g.bg.draw(screen)
	g.scene.Draw(screen)
	if g.cap != nil && g.cap.afterDraw(screen) {
		quitRequested = true
	}
	if g.rec != nil && g.rec.afterDraw(screen) {
		quitRequested = true
	}
}

func (g *Game) Layout(_, _ int) (int, int) { return ScreenW, ScreenH }

var quitRequested bool

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
	loadAssets()
	if *recordPath != "" {
		// told before a window opens, rather than after the game has loaded
		if err := checkRecordChar(*recordChar, characters); err != nil {
			exitUsage(err)
		}
		if err := findFFmpeg(); err != nil {
			log.Fatal(err)
		}
	}
	store.Load()
	// the scripted runs of a capture or a demo must not change the player's save
	store.ReadOnly = *captureDir != "" || *recordPath != ""
	initBlocks()
	if store.ReadOnly {
		// A capture or a demo plays no sound, so it does not open the audio device: on a
		// machine without one (a CI runner) opening it fails and ends the game.
		sound.SetMuted(true)
	} else {
		sound.Init()
	}

	ebiten.SetWindowTitle("Rabbit Run")
	// Shrink the window so it fits the monitor height.
	w, h := ScreenW, ScreenH
	if m := ebiten.Monitor(); m != nil {
		if _, mh := m.Size(); mh > 0 && mh*85/100 < h {
			h = mh * 85 / 100
			w = h * ScreenW / ScreenH
		}
	}
	ebiten.SetWindowSize(w, h)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	bg = newBackground()
	g := &Game{bg: bg}
	g.scene = newTitleScene()
	if *captureDir != "" {
		g.cap = &captureState{}
		sound.SetMuted(true)
		// Without vsync: held to the display's refresh, a window in the background got a
		// few frames a second and the capture took minutes. The game still runs at 60
		// ticks a second, so the pictures decoded in the background are in on time.
		ebiten.SetVsyncEnabled(false)
	}
	if *recordPath != "" {
		g.rec = &recorder{}
		if err := g.rec.start(g); err != nil {
			log.Fatal(err)
		}
		sound.SetMuted(true)
	}
	err := ebiten.RunGame(g)
	store.Flush() // the window was closed: whatever changed last is written
	if err != nil {
		log.Fatal(err)
	}
}
