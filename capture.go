package main

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/nao1215/rabbitrun/road"

	"github.com/hajimehoshi/ebiten/v2"
)

// writeBGMWavs writes 30 seconds of each song at each intensity stage to dir as WAV
// files (the --bgm-wav option).
func writeBGMWavs(dir string) {
	names := []string{"spring", "summer", "autumn", "winter"}
	for i := range len(names) * 3 {
		name := names[i/3]
		lv := i % 3
		style := name + "_" + []string{"0calm", "1groove", "2full"}[lv]
		m := newMusicStream(songs[name])
		m.setIntensity(lv)
		m.level = lv
		m.setBPM(intensityBPM[lv])
		m.curBPM = intensityBPM[lv]
		const sec = 30
		pcm := make([]byte, 8*sampleRate*sec)
		if _, err := m.Read(pcm); err != nil {
			log.Printf("cannot render %s: %v", style, err)
			continue
		}
		// float32 stereo -> 16-bit stereo WAV
		data := make([]byte, 0, 4*sampleRate*sec)
		for i := 0; i+4 <= len(pcm); i += 4 { // left and right interleaved
			v := math.Float32frombits(binary.LittleEndian.Uint32(pcm[i:]))
			sample := int16(max(-1, min(1, v)) * 32767)
			data = binary.LittleEndian.AppendUint16(data, uint16(sample)) //nolint:gosec // G115: intentional two's complement reinterpretation of a signed PCM sample
		}
		// data holds 30 seconds of 16-bit stereo audio (about 5 MB), far below the uint32 limit of a WAV header.
		dataLen := uint32(len(data)) //nolint:gosec // G115: len(data) is a fixed ~5 MB, well within uint32
		h := []byte("RIFF")
		h = binary.LittleEndian.AppendUint32(h, 36+dataLen)
		h = append(h, "WAVEfmt "...)
		h = binary.LittleEndian.AppendUint32(h, 16)
		h = binary.LittleEndian.AppendUint16(h, 1)
		h = binary.LittleEndian.AppendUint16(h, 2)
		h = binary.LittleEndian.AppendUint32(h, sampleRate)
		h = binary.LittleEndian.AppendUint32(h, sampleRate*4)
		h = binary.LittleEndian.AppendUint16(h, 4)
		h = binary.LittleEndian.AppendUint16(h, 16)
		h = append(h, "data"...)
		h = binary.LittleEndian.AppendUint32(h, dataLen)
		p := filepath.Join(dir, style+".wav")
		if err := os.WriteFile(p, append(h, data...), 0o600); err != nil {
			log.Printf("cannot write %s: %v", p, err)
			continue
		}
		if _, err := fmt.Println(p); err != nil {
			log.Print(err)
		}
	}
}

type recorder struct {
	frame int
	drawn bool
	cmd   *exec.Cmd
	pipe  io.WriteCloser
	pix   []byte
}

// start opens a self-playing game on the main character and starts ffmpeg.
func (r *recorder) start(g *Game) error {
	c := characters[defaultCharIndex()]
	for _, ch := range characters {
		if ch.ID == *recordChar {
			c = ch
		}
	}
	s := newPlayScene(c)
	s.auto = &autoPlayer{}
	if st := *recordStage; st > 1 {
		s.eng.G.StartAt((st-1)*road.Courses + 1)
	}
	g.SetScene(s)
	r.drawn = true
	// Draw as fast as possible: a hidden window would otherwise be held to a few frames a second.
	ebiten.SetVsyncEnabled(false)
	ebiten.SetTPS(ebiten.SyncWithFPS)
	r.cmd = exec.CommandContext(context.Background(), "ffmpeg", "-y", "-loglevel", "error", "-f", "rawvideo", "-pix_fmt", "rgba", //nolint:gosec // G204: fixed arguments and the path the user passed
		"-s", fmt.Sprintf("%dx%d", ScreenW, ScreenH), "-r", "30", "-i", "-",
		"-c:v", "libx264", "-pix_fmt", "yuv420p", "-crf", "23", *recordPath)
	pipe, err := r.cmd.StdinPipe()
	if err != nil {
		return err
	}
	r.pipe = pipe
	r.pix = make([]byte, 4*ScreenW*ScreenH)
	return r.cmd.Start()
}

// skipUpdate holds the game until the last frame was drawn, so the video has every
// other frame (30 per second) even when encoding is slower than the game.
func (r *recorder) skipUpdate() bool {
	if !r.drawn {
		return true
	}
	r.drawn = false
	r.frame++
	return false
}

// afterDraw sends the frame to ffmpeg and reports whether the recording is over.
func (r *recorder) afterDraw(screen *ebiten.Image) bool {
	r.drawn = true
	if r.frame%2 == 0 {
		screen.ReadPixels(r.pix)
		if _, err := r.pipe.Write(r.pix); err != nil {
			log.Printf("cannot write a frame: %v", err)
			return true
		}
	}
	if r.frame < *recordSeconds*60 {
		return false
	}
	if err := r.pipe.Close(); err != nil {
		log.Print(err)
	}
	if err := r.cmd.Wait(); err != nil {
		log.Printf("ffmpeg: %v", err)
	}
	return true
}

type captureStep struct {
	name  string
	setup func(g *Game)
	wait  int
}

var captureSteps = []captureStep{
	{"title", func(g *Game) { g.SetScene(newTitleScene()) }, 30},
	{"title_reveal_grey", titleRevealScene, revealColorStart + 5}, // a new character comes in grey
	{"title_reveal", titleRevealScene, revealWordsAt + 40},
	{"title_word", func(g *Game) {
		s := newTitleScene()
		s.reveal, s.word = -1, true
		g.SetScene(s)
	}, wordWait + 10}, // in her colors, with the words
	{"title_group", func(g *Game) { s := newTitleScene(); s.group = true; g.SetScene(s) }, 30},
	{"select", func(g *Game) { g.SetScene(newCharSelectScene(modePlay)) }, 40},
	{"play", func(g *Game) {
		s := newPlayScene(characters[defaultCharIndex()])
		s.ready = 0
		s.auto = &autoPlayer{} // a few seconds of play, so the road is in motion
		g.SetScene(s)
	}, 300},
	{"play_panic", func(g *Game) {
		s := newPlayScene(characters[defaultCharIndex()])
		s.ready = 0
		s.auto = &autoPlayer{}
		s.eng.G.Lives = 1
		s.eng.G.StartAt(12) // a narrow, fast road on the last life
		g.SetScene(s)
	}, 240},
	{"play_flash", func(g *Game) {
		c := characters[defaultCharIndex()]
		s := newPlayScene(c)
		s.ready = 0
		s.auto = &autoPlayer{}
		// The second stage: the first illustration with a picture is the background.
		s.eng.G.StartAt(road.Courses + 1)
		for i := range c.CGs {
			if c.CGs[i].HasImage() {
				s.setStageCG(&c.CGs[i])
				break
			}
		}
		g.SetScene(s)
	}, 40},
	{"play_miss", func(g *Game) {
		s := newPlayScene(characters[defaultCharIndex()])
		s.ready = 0
		for x := range road.W {
			s.eng.G.Rows[road.PlayerRow-1][x].Wall = road.CourseColors[0] // a wall she cannot avoid
		}
		g.SetScene(s)
	}, 75},
	{"play_vault", func(g *Game) {
		s := newPlayScene(characters[defaultCharIndex()])
		s.ready = 0
		e := s.eng
		e.G.StartAt(3)     // the first vault's course
		e.G.Safe = 1 << 30 // run ahead to the vault (walls pass through)
		for f := 0; f < 60*60 && !vaultOnScreen(e.G); f++ {
			e.Tick(false)
		}
		e.G.Safe = 0
		e.Events = e.Events[:0]
		g.SetScene(s)
	}, 2},
	{"play_retry", func(g *Game) {
		s := newPlayScene(characters[defaultCharIndex()])
		e := s.eng
		e.G.StartAt(6) // a miss just into course 2-2
		e.G.Missed = true
		e.Restart() // back to the end of 2-1
		s.ready = readyFr
		s.restartBackground()
		s.handleEvents()
		g.SetScene(s)
	}, 20},
	{"play_feast", func(g *Game) {
		s := newPlayScene(characters[defaultCharIndex()])
		s.ready = 0
		e := s.eng
		e.G.StartAt(5) // course 5 has a feast
		e.G.Safe = 1 << 30
		for f := 0; f < 60*60 && !feastOnScreen(e.G); f++ {
			e.Tick(false)
		}
		e.G.Safe = 0
		e.Events = e.Events[:0]
		s.restartBackground()
		g.SetScene(s)
	}, 2},
	{"play_intro", introScene, 40},
	{"play_show", introScene, introFrames + showHold/2},
	{"play_show_cutin", introScene, introFrames + showHold + cutinFrames/2},
	{"play_show_break", introScene, introFrames + showHold + cutinFrames + 20},
	{"play_break", func(g *Game) {
		s := newPlayScene(characters[defaultCharIndex()])
		s.ready = 0
		for y := range road.Rows {
			for _, x := range []int{0, 1, 2, 6, 7, 8} {
				s.eng.G.Rows[y][x].Wall = road.CourseColors[y%road.Courses]
			}
		}
		s.useHammer()
		g.SetScene(s)
	}, cutinFrames + 25},
	{"play_cutin", func(g *Game) { cutinScene(g, defaultCharIndex()) }, 30},
	{"play_cutin_2", func(g *Game) { cutinScene(g, (defaultCharIndex()+1)%len(characters)) }, 30},
	{"play_cutin_3", func(g *Game) { cutinScene(g, (defaultCharIndex()+2)%len(characters)) }, 30},
	{"play_cutin_4", func(g *Game) { cutinScene(g, (defaultCharIndex()+3)%len(characters)) }, 30},
	{"allclear_road", allClearScene, 40},    // out of the last course onto the open road
	{"allclear", allClearScene, 330},        // the ending on the last illustration
	{"gameover_curtain", gameOverScene, 55}, // the curtain coming down
	{"gameover", gameOverScene, 130},
	{"gallery", func(g *Game) { g.SetScene(newGalleryScene()) }, 120},
}

type captureState struct {
	step, frame int
}

func (c *captureState) update(g *Game) {
	if c.step >= len(captureSteps) {
		return
	}
	if c.frame == 0 {
		captureSteps[c.step].setup(g)
	}
	c.frame++
}

func (c *captureState) afterDraw(screen *ebiten.Image) bool {
	if c.step >= len(captureSteps) {
		return true
	}
	st := captureSteps[c.step]
	if c.frame < st.wait {
		return false
	}
	img := image.NewRGBA(image.Rect(0, 0, ScreenW, ScreenH))
	screen.ReadPixels(img.Pix)
	p := filepath.Join(*captureDir, st.name+".png")
	if err := writePNG(p, img); err != nil {
		log.Printf("cannot save %s: %v", p, err)
	} else if _, err := fmt.Println(p); err != nil {
		log.Print(err)
	}
	c.step++
	c.frame = 0
	return c.step >= len(captureSteps)
}

// writePNG encodes img as a PNG file at p.
func writePNG(p string, img image.Image) error {
	f, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600) //nolint:gosec // G304: p is under the directory the user passed with -capture
	if err != nil {
		return err
	}
	if err := png.Encode(f, img); err != nil {
		return errors.Join(err, f.Close())
	}
	return f.Close()
}

// cutinScene swings a hammer so the capture shows the cut-in of character i.
func cutinScene(g *Game, i int) {
	s := newPlayScene(characters[i])
	s.ready = 0
	s.useHammer()
	g.SetScene(s)
}

// vaultOnScreen reports whether a vault's two prizes are in the upper half of the road.
func vaultOnScreen(g *road.Game) bool {
	for y := 3; y < road.Rows/2; y++ {
		r := g.Rows[y]
		for x := 1; x+2 < road.W; x++ {
			if r[x].Sweet != road.SweetNone && r[x+1].Sweet != road.SweetNone && r[x-1].Wall != 0 && r[x+2].Wall != 0 &&
				g.Rows[y-1][x].Wall != 0 && g.Rows[y+1][x].Wall != 0 {
				return true
			}
		}
	}
	return false
}

// feastOnScreen reports whether the rows full of sweets fill the middle of the road.
func feastOnScreen(g *road.Game) bool {
	n := 0
	for y := 2; y < road.PlayerRow-2; y++ {
		full := 0
		for _, c := range g.Rows[y] {
			if c.Sweet != road.SweetNone {
				full++
			}
		}
		if full >= 6 {
			n++
		}
	}
	return n >= 8
}

// titleRevealScene is the title bringing in the secret character as just unlocked.
func titleRevealScene(g *Game) {
	s := newTitleScene()
	for i, c := range characters {
		if c.Secret {
			s.reveal = i
		}
	}
	g.SetScene(s)
}

// allClearScene runs the end of the last course by itself (through the walls) up to where
// the open road after it begins.
func allClearScene(g *Game) {
	s := newPlayScene(characters[defaultCharIndex()])
	s.ready = 0
	e := s.eng
	e.G.StartAt(GameCourses) // the road of the last course, from its start
	e.G.Safe = 1 << 30
	for f := 0; f < 60*60 && e.G.Level <= GameCourses; f++ {
		e.Tick(false)
	}
	s.handleEvents()
	g.SetScene(s)
}

// gameOverScene runs a game with no life left into the first wall: the full game over.
func gameOverScene(g *Game) {
	s := newPlayScene(characters[defaultCharIndex()])
	s.ready = 0
	s.eng.G.Lives = 0 // no life left: the first wall ends it
	for !s.eng.Over() {
		s.eng.Tick(true) // nobody steers
	}
	g.SetScene(s)
}

// introScene starts a run with its intro and the hammer show after it.
func introScene(g *Game) {
	s := newPlayScene(characters[defaultCharIndex()])
	s.startIntro()
	g.SetScene(s)
}
