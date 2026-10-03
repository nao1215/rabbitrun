package main

import (
	"fmt"
	"image"
	"image/color"
	"log"
	"math"
	"os"

	flag "github.com/spf13/pflag"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// Portrait screen (4:5). The board fills the full height with no top or bottom margin.
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
	flag.Usage = printUsage
	flag.Parse()
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
	for _, dir := range []string{*bgmWavDir, *captureDir} {
		if dir == "" {
			continue
		}
		if err := os.MkdirAll(dir, 0o750); err != nil {
			log.Fatal(err)
		}
	}
	if *bgmWavDir != "" {
		writeBGMWavs(*bgmWavDir)
		return
	}
	loadAssets()
	if *resetSaveFlag {
		if err := resetSave(); err != nil {
			log.Fatal(err)
		}
		log.Printf("the save data was reset (the old one is kept as %s.bak)", savePath())
	}
	loadSave()
	initBlocks()
	initAudio()

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
		audioMuted = true
	}
	if *recordPath != "" {
		g.rec = &recorder{}
		if err := g.rec.start(g); err != nil {
			log.Fatal(err)
		}
		audioMuted = true
	}
	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}

// ---- Background: a flat solid color chosen by scene and expression, faded in slowly ----

// Pop color palette for the flat backgrounds.
var (
	popPink     = color.NRGBA{0xf6, 0xb3, 0xc6, 0xff}
	popYellow   = color.NRGBA{0xf8, 0xdc, 0x6c, 0xff}
	popOrange   = color.NRGBA{0xf4, 0x95, 0x55, 0xff}
	popMint     = color.NRGBA{0xb8, 0xe6, 0xda, 0xff}
	popLavender = color.NRGBA{0xd4, 0xc4, 0xf0, 0xff}
	popCream    = color.NRGBA{0xfb, 0xf1, 0xd8, 0xff}
	popGray     = color.NRGBA{0xc4, 0xc0, 0xcc, 0xff}
)

type background struct {
	cur, target [3]float64
	image       string // assets/ui/<image>.jpg drawn underneath (color only if empty)
}

func (b *background) setImage(name string) { b.image = name }

func newBackground() *background {
	b := &background{}
	b.set(popPink)
	b.cur = b.target
	return b
}

func (b *background) set(c color.NRGBA) {
	b.target = [3]float64{float64(c.R), float64(c.G), float64(c.B)}
}

func (b *background) update() {
	for i := range b.cur {
		b.cur[i] += (b.target[i] - b.cur[i]) * 0.06
	}
}

func (b *background) color() color.NRGBA {
	return color.NRGBA{uint8(b.cur[0]), uint8(b.cur[1]), uint8(b.cur[2]), 0xff}
}

func (b *background) draw(screen *ebiten.Image) {
	screen.Fill(b.color())
	if img := uiImage(b.image); img != nil {
		drawImageCover(screen, img, 0, 0, ScreenW, ScreenH, 1)
	}
}

// bg is the current background. Scenes set its color.
var bg *background

// ---- Drawing helpers ----

func roundRectPath(x, y, w, h, r float32) *vector.Path {
	p := &vector.Path{}
	p.MoveTo(x+r, y)
	p.LineTo(x+w-r, y)
	p.ArcTo(x+w, y, x+w, y+r, r)
	p.LineTo(x+w, y+h-r)
	p.ArcTo(x+w, y+h, x+w-r, y+h, r)
	p.LineTo(x+r, y+h)
	p.ArcTo(x, y+h, x, y+h-r, r)
	p.LineTo(x, y+r)
	p.ArcTo(x, y, x+r, y, r)
	p.Close()
	return p
}

func fillRoundRect(dst *ebiten.Image, x, y, w, h, r float32, clr color.Color) {
	op := &vector.DrawPathOptions{AntiAlias: true}
	op.ColorScale.ScaleWithColor(clr)
	vector.FillPath(dst, roundRectPath(x, y, w, h, r), nil, op)
}

func strokeRoundRect(dst *ebiten.Image, x, y, w, h, r, width float32, clr color.Color) {
	op := &vector.DrawPathOptions{AntiAlias: true}
	op.ColorScale.ScaleWithColor(clr)
	vector.StrokePath(dst, roundRectPath(x, y, w, h, r), &vector.StrokeOptions{Width: width}, op)
}

var (
	panelFill = color.NRGBA{0xff, 0xfe, 0xfb, 0xff}
	textMain  = color.NRGBA{0x3c, 0x2a, 0x2e, 0xff}
	candyPink = color.NRGBA{0xf0, 0x5a, 0x8c, 0xff}
)

// shadowColor returns a flat shadow color derived by darkening the background.
func shadowColor() color.NRGBA {
	c := bg.color()
	return color.NRGBA{uint8(float64(c.R) * 0.78), uint8(float64(c.G) * 0.72), uint8(float64(c.B) * 0.76), 0xff}
}

// drawPanel draws a white rounded panel with a flat shadow offset to the lower right.
func drawPanel(dst *ebiten.Image, x, y, w, h float32) {
	fillRoundRect(dst, x+6, y+8, w, h, 18, shadowColor())
	fillRoundRect(dst, x, y, w, h, 18, panelFill)
}

// drawImageFit draws img centered at the largest size that fits the box.
func drawImageFit(dst, img *ebiten.Image, x, y, w, h float64, alpha float32) {
	iw, ih := float64(img.Bounds().Dx()), float64(img.Bounds().Dy())
	s := math.Min(w/iw, h/ih)
	drawImageScaled(dst, img, x+(w-iw*s)/2, y+(h-ih*s)/2, s, alpha)
}

func drawImageScaled(dst, img *ebiten.Image, x, y, s float64, alpha float32) {
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(s, s)
	op.GeoM.Translate(x, y)
	op.ColorScale.ScaleAlpha(alpha)
	op.Filter = ebiten.FilterLinear
	dst.DrawImage(img, op)
}

func dimScreen(dst *ebiten.Image, a uint8) {
	c := bg.color()
	vector.FillRect(dst, 0, 0, ScreenW, ScreenH, color.NRGBA{c.R, c.G, c.B, a}, false)
}

// drawTextOutline draws centered text with a white outline so it stays readable over images.
func drawTextOutline(dst *ebiten.Image, str string, x, y, size float64, clr color.Color) {
	w := math.Max(2, size/12)
	for i := range 12 {
		t := float64(i) / 12 * 2 * math.Pi
		drawText(dst, str, x+math.Cos(t)*w, y+math.Sin(t)*w, size, color.White)
	}
	drawText(dst, str, x, y, size, clr)
}

// drawTextOutlineColor draws centered text in fill with an outline of the given color, both
// faded by alpha (dark outlines read on a dark screen).
func drawTextOutlineColor(dst *ebiten.Image, str string, x, y, size float64, fill, outline color.Color, alpha float32) {
	fade := func(c color.Color) color.Color {
		n := color.NRGBAModel.Convert(c).(color.NRGBA) //nolint:errcheck,forcetypeassert // NRGBAModel always returns NRGBA
		n.A = uint8(float32(n.A) * alpha)
		return n
	}
	w := math.Max(2, size/12)
	for i := range 12 {
		t := float64(i) / 12 * 2 * math.Pi
		drawText(dst, str, x+math.Cos(t)*w, y+math.Sin(t)*w, size, fade(outline))
	}
	drawText(dst, str, x, y, size, fade(fill))
}

// drawMenu draws a vertical menu. The selected item is pink and a little larger.
func drawMenu(dst *ebiten.Image, items []string, sel int, y, size float64) {
	drawMenuAt(dst, items, sel, ScreenW/2, y, size)
}

// drawMenuAt draws a vertical menu centered at cx.
func drawMenuAt(dst *ebiten.Image, items []string, sel int, cx, y, size float64) {
	drawMenuOn(dst, items, sel, cx, y, size, textMain, color.White)
}

// drawMenuOn draws a vertical menu centered at cx, the items not selected in normal, all
// outlined in outline (light items with a dark outline read on a dark screen).
func drawMenuOn(dst *ebiten.Image, items []string, sel int, cx, y, size float64, normal, outline color.Color) {
	for i, s := range items {
		yy := y + float64(i)*size*1.7
		clr, sz := normal, size
		if i == sel {
			clr, sz = candyPink, size*1.15
			yy -= (sz - size) / 2
		}
		drawTextOutlineColor(dst, s, cx, yy, sz, clr, outline, 1)
	}
}

// ---- Screen artwork (assets/ui/) ----

var uiCache = map[string]*ebiten.Image{}

// imageExts lists the image file extensions tried in order when looking up artwork.
var imageExts = []string{".jpg", ".png"}

// uiImage reads assets/ui/<name>.jpg (or .png), returning nil if missing.
func uiImage(name string) *ebiten.Image {
	if name == "" {
		return nil
	}
	if img, ok := uiCache[name]; ok {
		return img
	}
	var img *ebiten.Image
	for _, ext := range imageExts {
		if dec, err := decodeAsset("assets/ui/" + name + ext); err == nil {
			img = ebiten.NewImageFromImage(dec)
			break
		}
	}
	uiCache[name] = img
	return img
}

// drawImageCoverTop scales img to fill the box. Overflow is cropped evenly left and right,
// and from the bottom vertically (top-aligned so faces stay visible).
func drawImageCoverTop(dst, img *ebiten.Image, x, y, w, h float64, alpha float32) {
	iw, ih := float64(img.Bounds().Dx()), float64(img.Bounds().Dy())
	s := math.Max(w/iw, h/ih)
	sw, sh := w/s, h/s
	sx := (iw - sw) / 2
	sub, ok := img.SubImage(image.Rect(int(sx), 0, int(sx+sw), int(sh))).(*ebiten.Image)
	if !ok {
		return
	}
	drawImageScaled(dst, sub, x, y, s, alpha)
}

// drawImageCover scales img to fill the box and crops the overflow.
func drawImageCover(dst, img *ebiten.Image, x, y, w, h float64, alpha float32) {
	iw, ih := float64(img.Bounds().Dx()), float64(img.Bounds().Dy())
	s := math.Max(w/iw, h/ih)
	sw, sh := w/s, h/s
	sx, sy := (iw-sw)/2, (ih-sh)/2
	sub, ok := img.SubImage(image.Rect(int(sx), int(sy), int(sx+sw), int(sy+sh))).(*ebiten.Image)
	if !ok {
		return
	}
	drawImageScaled(dst, sub, x, y, s, alpha)
}

var maskCache = map[[3]int]*ebiten.Image{}

// roundedMask rounds the corners of img by radius r, in place, and returns it. The mask
// (a white rounded rectangle) is made once per size and applied with a blend: filling the
// rounded path anew every frame was most of the cost of the character's frame.
func roundedMask(img *ebiten.Image, r float32) *ebiten.Image {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	key := [3]int{w, h, int(r)}
	mask, ok := maskCache[key]
	if !ok {
		mask = ebiten.NewImage(w, h)
		fillRoundRect(mask, 0, 0, float32(w), float32(h), r, color.White)
		maskCache[key] = mask
	}
	img.DrawImage(mask, &ebiten.DrawImageOptions{Blend: ebiten.BlendDestinationIn})
	return img
}
