package main

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

// titleLogoLines are the two lines of the title logo: the letters, their size and
// the candy color each letter takes.
var titleLogoLines = []struct {
	word  string
	size  float64
	kinds []Kind
}{
	{"RABBIT", 96, []Kind{KindStrawberry, KindOrange, KindLemon, KindMelon, KindSoda, KindGrape}},
	{"RUN", 140, []Kind{KindBlueberry, KindStrawberry, KindOrange}},
}

// logoShadow is the soft drop shadow under the title letters.
var logoShadow = color.NRGBA{0xb0, 0x4a, 0x78, 0x70}

// drawTitleLogo draws "RABBIT RUN" in the pop font, each letter in a candy color
// with a thick white outline and a drop shadow. The RUN line is centered on cy;
// scale shrinks the whole logo.
func drawTitleLogo(dst *ebiten.Image, cx, cy, scale float64) {
	last := titleLogoLines[len(titleLogoLines)-1]
	top := cy - last.size*scale*0.62
	tops := make([]float64, len(titleLogoLines))
	tops[len(tops)-1] = top
	for i := len(titleLogoLines) - 2; i >= 0; i-- {
		tops[i] = tops[i+1] - titleLogoLines[i].size*scale*1.02
	}
	for li, line := range titleLogoLines {
		size := line.size * scale
		f := face(size, line.word)
		spacing := size * 0.04
		// Letter widths from the shaped word (the logo words are ASCII, one byte per letter).
		widths := make([]float64, 0, len(line.word))
		total := 0.0
		for i := range len(line.word) {
			w := text.AdvanceAt(line.word, i+1, f) - text.AdvanceAt(line.word, i, f)
			widths = append(widths, w)
			total += w + spacing
		}
		total -= spacing
		x := cx - total/2
		for i, r := range line.word {
			s := string(r)
			lx := x + widths[i]/2
			ly := tops[li]
			drawCandyText(dst, s, lx, ly, size, line.kinds[i])
			x += widths[i] + spacing
		}
	}
}

// drawCandyText draws s centered at cx (top at y) in the pop font, in the candy color
// of the given kind with a thick white outline and a soft shadow, like the title logo.
func drawCandyText(dst *ebiten.Image, s string, cx, y, size float64, kind Kind) {
	drawText(dst, s, cx+size*0.05, y+size*0.07, size, logoShadow)
	ow := size / 11
	for k := range 16 {
		t := float64(k) / 16 * 2 * math.Pi
		drawText(dst, s, cx+math.Cos(t)*ow, y+math.Sin(t)*ow, size, color.White)
	}
	drawText(dst, s, cx, y, size, kindColors[kind])
}

// logoLineKinds are the candy colors the letters of a one-line logo take in turn.
var logoLineKinds = []Kind{KindStrawberry, KindOrange, KindLemon, KindMelon, KindSoda, KindGrape, KindBlueberry}

// drawLogoLine draws str on one line in the logo's candy letters (each letter its own
// color, white outline and shadow), centered on cx with its top at y.
func drawLogoLine(dst *ebiten.Image, str string, cx, y, size float64) {
	f := face(size, str)
	spacing := size * 0.04
	widths := make([]float64, 0, len(str))
	total := 0.0
	for i := range len(str) {
		w := text.AdvanceAt(str, i+1, f) - text.AdvanceAt(str, i, f)
		widths = append(widths, w)
		total += w + spacing
	}
	x := cx - (total-spacing)/2
	k := 0
	for i, r := range str {
		if r != ' ' {
			drawCandyText(dst, string(r), x+widths[i]/2, y, size, logoLineKinds[k%len(logoLineKinds)])
			k++
		}
		x += widths[i] + spacing
	}
}
