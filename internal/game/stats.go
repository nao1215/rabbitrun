package game

import (
	"fmt"
	"image/color"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/nao1215/rabbitrun/internal/gfx"
)

// The frame meter of the browser build (the page opened with ?fps, statsAsked): the
// ticks and frames a second as ebiten counts them, and the longest Update and Draw of
// the last second, at the bottom of the screen. It is for telling on a phone whether the
// game keeps up; without ?fps nothing of it runs.

// showStats is set when the meter is asked for.
var showStats = statsAsked()

// frameStats is what the meter shows: measured over a second, then shown for the next.
type frameStats struct {
	since           time.Time
	updMax, drawMax time.Duration
	drawSum         time.Duration
	draws           int
	text            string
}

// timeUpdate measures an Update for the meter: call the function it returns at its end.
func (g *Game) timeUpdate() func() {
	start := time.Now()
	return func() {
		s := &g.stats
		s.updMax = max(s.updMax, time.Since(start))
		if time.Since(s.since) < time.Second {
			return
		}
		drawAvg := ms(s.drawSum) / float64(max(1, s.draws))
		s.text = fmt.Sprintf("TPS %.1f  FPS %.1f\nmax update %.1fms  draw %.1fms (avg %.1f)",
			ebiten.ActualTPS(), ebiten.ActualFPS(), ms(s.updMax), ms(s.drawMax), drawAvg)
		publishStats(ebiten.ActualTPS(), ebiten.ActualFPS(), ms(s.updMax), ms(s.drawMax), drawAvg)
		s.since, s.updMax, s.drawMax, s.drawSum, s.draws = time.Now(), 0, 0, 0, 0
	}
}

// timeDraw measures a Draw for the meter and draws the meter over it: call the function
// it returns at the end of the Draw.
func (g *Game) timeDraw(screen *ebiten.Image) func() {
	start := time.Now()
	return func() {
		d := time.Since(start)
		g.stats.drawMax = max(g.stats.drawMax, d)
		g.stats.drawSum += d
		g.stats.draws++
		g.drawStats(screen)
	}
}

// drawStats draws the meter's text at the bottom of the screen, on a dark band. It is
// drawn in the game's font: ebitenutil's debug print brought net/http into the
// WebAssembly file, which grew by 2 MB.
func (g *Game) drawStats(screen *ebiten.Image) {
	if g.stats.text == "" {
		return
	}
	const size, bandH = 22.0, 68
	vector.FillRect(screen, 0, ScreenH-bandH, ScreenW, bandH, color.NRGBA{0, 0, 0, 0xa0}, false)
	gfx.DrawText(screen, g.stats.text, ScreenW/2, ScreenH-bandH+4, size, color.White)
}

func ms(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }
