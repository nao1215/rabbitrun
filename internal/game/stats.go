package game

import (
	"fmt"
	"image/color"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

// The frame meter of the browser build (the page opened with ?fps, statsAsked): the
// ticks and frames a second as ebiten counts them, and the longest Update and Draw of
// the last second, in the bottom left corner. It is for telling on a phone whether the
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
	img             *ebiten.Image // the meter's text, drawn at its own size (drawStats)
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

// statsScale is how much the meter's debug font is enlarged: at its own size it was too
// small to read on a phone, where the screen is shown at about half its size.
const statsScale = 2

// drawStats draws the meter's text in the bottom left corner, on a dark band.
func (g *Game) drawStats(screen *ebiten.Image) {
	if g.stats.text == "" {
		return
	}
	if g.stats.img == nil {
		g.stats.img = ebiten.NewImage(ScreenW/statsScale, 36)
	}
	img := g.stats.img
	img.Fill(color.NRGBA{0, 0, 0, 0xa0})
	ebitenutil.DebugPrintAt(img, g.stats.text, 4, 2)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(statsScale, statsScale)
	op.GeoM.Translate(0, ScreenH-36*statsScale)
	screen.DrawImage(img, op)
}

func ms(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }
