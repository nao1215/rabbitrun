// Package game is Rabbit Run itself: the screens (title, character select, play with its
// hammer show, miss and ending, the gallery), the background and the gummy blocks, and the
// scripted runs of --capture and --record-demo. It drives the other packages: the timed
// road (engine), the characters, the controls (input), the sound and the save data.
package game

import (
	"io/fs"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/nao1215/rabbitrun/internal/input"
	"github.com/nao1215/rabbitrun/internal/sound"
)

// Portrait screen (4:5).
const (
	ScreenW = 720
	ScreenH = 900
)

// Scene is one screen of the game.
type Scene interface {
	Update(g *Game)
	Draw(screen *ebiten.Image)
}

// Game runs the screens; it is the ebiten.Game the window runs.
type Game struct {
	in    input.Input
	scene Scene
	frame int
	bg    *background
	cap   *captureState
	rec   *recorder
}

// Options are what the command line asks of a run of the game.
type Options struct {
	// CaptureDir, when set, saves a screenshot of every screen there and ends the game.
	CaptureDir string
	// Debug unlocks every character, portrait and illustration for this run (the save data
	// is unchanged).
	Debug bool
	// RecordPath, when set, plays a demo by itself and records it to this video file with
	// ffmpeg, then ends the game. RecordChar is the character's ID (the main character
	// when empty), RecordStage the stage it starts at (1 for the first) and RecordSeconds
	// how long it lasts. RecordExtra records it on the extra stages.
	RecordPath    string
	RecordChar    string
	RecordStage   int
	RecordSeconds int
	RecordExtra   bool
}

// debugMode unlocks the whole gallery and every character (Options.Debug).
var debugMode bool

// Load reads the fonts and the characters from fsys, the assets directory. It is called
// once, before New.
func Load(fsys fs.FS) error { return loadAssets(fsys) }

// CharacterIDs are the IDs of the characters Load found, in the order of the select screen.
func CharacterIDs() []string {
	ids := make([]string, len(characters))
	for i, c := range characters {
		ids[i] = c.ID
	}
	return ids
}

// New sets up a run of the game: the save data, the gummy blocks, the sound, and the
// scripted capture or demo recording when opts asks for one. Close it when the window
// closes.
func New(opts Options) (*Game, error) {
	openSave(opts)
	initBlocks()
	if opts.CaptureDir != "" || opts.RecordPath != "" {
		// A capture or a demo plays no sound, so it does not open the audio device: on a
		// machine without one (a CI runner) opening it fails and ends the game.
		sound.SetMuted(true)
	} else {
		sound.Init()
	}
	bg = newBackground()
	g := &Game{bg: bg}
	g.scene = newTitleScene()
	if opts.CaptureDir != "" {
		g.cap = &captureState{dir: opts.CaptureDir}
		// Without vsync: held to the display's refresh, a window in the background got a
		// few frames a second and the capture took minutes. The game still runs at 60
		// ticks a second, so the pictures decoded in the background are in on time.
		ebiten.SetVsyncEnabled(false)
	}
	if opts.RecordPath != "" {
		g.rec = &recorder{path: opts.RecordPath, char: opts.RecordChar, stage: opts.RecordStage, seconds: opts.RecordSeconds, extra: opts.RecordExtra}
		if err := g.rec.start(g); err != nil {
			return nil, err
		}
	}
	return g, nil
}

// openSave loads the save data for a run of the game asked for by opts.
func openSave(opts Options) {
	debugMode = opts.Debug
	store.Load()
	// The scripted runs of a capture or a demo must not change the player's save, and
	// neither does --debug: what it opens is for this run only, and a course cleared or a
	// character announced while everything is open would count in the real game.
	store.ReadOnly = opts.CaptureDir != "" || opts.RecordPath != "" || opts.Debug
	forgetUnearnedAnnouncements()
}

// forgetUnearnedAnnouncements drops the secret characters saved as announced although the
// save data has not earned them. Before --debug left the save alone, its run brought the
// secret character in on the title and saved her as announced; she cannot have been
// announced for real before she was earned (the progress only grows), so such an entry
// came from --debug, and keeping it would hide her arrival once she is earned.
func forgetUnearnedAnnouncements() {
	if len(store.Data.Announced) == 0 || secretEarned(characters, store.Data) {
		return
	}
	for _, c := range characters {
		if c.Secret && store.Data.Announced[c.ID] {
			delete(store.Data.Announced, c.ID)
			store.Mark()
		}
	}
}

// Close writes whatever changed last in the save data (the window was closed).
func (g *Game) Close() { store.Flush() }

// SetScene switches to the scene s. The scene left frees what it holds on the GPU, if it
// has a release method.
func (g *Game) SetScene(s Scene) {
	if r, ok := g.scene.(interface{ release() }); ok && g.scene != s {
		r.release()
	}
	store.Flush()
	g.scene = s
}

// Update runs a frame of the game.
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

// Draw draws the frame.
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

// Layout keeps the screen at its own size; the window scales it.
func (g *Game) Layout(_, _ int) (int, int) { return ScreenW, ScreenH }

var quitRequested bool
