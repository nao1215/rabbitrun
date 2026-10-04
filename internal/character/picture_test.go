package character

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strconv"
	"testing"
	"testing/fstest"
	"time"
)

// figurePicture is a 100x200 standing figure on a transparent ground: a pair of thin ears
// (x 45-48, y 0-9), a head (x 35-64, y 10-49) and a body (x 40-59, y 50-195).
func figurePicture() *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, 100, 200))
	fill := func(x0, y0, x1, y1 int) {
		for y := y0; y < y1; y++ {
			for x := x0; x < x1; x++ {
				img.SetNRGBA(x, y, color.NRGBA{0xf0, 0x90, 0xa0, 0xff})
			}
		}
	}
	fill(45, 0, 49, 10)
	fill(35, 10, 65, 50)
	fill(40, 50, 60, 196)
	return img
}

// figurePNG is figurePicture encoded as a PNG file.
func figurePNG(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, figurePicture()); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// testCharacter reads a character from a made-up assets directory: portraits normal and
// happy have a picture and happy_2 has none; illustration cg1 has one and cg2 has none;
// the select picture is there, the cut-in is not.
func testCharacter(t *testing.T) *Character {
	t.Helper()
	pic := figurePNG(t)
	fsys := fstest.MapFS{
		"characters/x/game.json": {Data: []byte(`{"id":"x","expressions":[{"id":"normal","state":"normal"},
			{"id":"happy","state":"happy"},{"id":"happy_2","state":"happy"}],
			"select":{"id":"select"},"cgs":[{"id":"cg1","score":1,"title":"one"},{"id":"cg2","score":2,"title":"two"}]}`)},
		"characters/x/images/normal.png": {Data: pic},
		"characters/x/images/happy.png":  {Data: pic},
		"characters/x/images/select.png": {Data: pic},
		"characters/x/images/cg1.png":    {Data: pic},
	}
	chars, err := Read(fsys)
	if err != nil || len(chars) != 1 {
		t.Fatalf("the test character did not load: %v", err)
	}
	return chars[0]
}

// wantFigure is the figure of figurePicture.
var wantFigure = Figure{Box: image.Rect(35, 0, 65, 196), BodyX: 49}

func TestPortraitLoadsAndReleases(t *testing.T) { //nolint:paralleltest // uses the figure cache
	c := testCharacter(t)
	e := c.Expression(ExprNormal)
	img := e.Img()
	if img == nil || img.Bounds().Dx() != 100 || img.Bounds().Dy() != 200 {
		t.Fatalf("the portrait loaded as %v", img)
	}
	if e.Img() != img {
		t.Fatal("the portrait loaded twice")
	}
	if f, ok := KnownFigure(img); !ok || f != wantFigure || FigureOf(img) != wantFigure {
		t.Fatalf("figure %+v (measured %v), want %+v", f, ok, wantFigure)
	}
	e.ReleaseImg()
	if e.Image != nil {
		t.Fatal("ReleaseImg kept the portrait")
	}
	if _, ok := KnownFigure(img); ok {
		t.Fatal("the figure of a released portrait is kept")
	}
}

func TestPrefetchedPortraitsAreUploaded(t *testing.T) { //nolint:paralleltest // uses the prefetch queue
	c := testCharacter(t)
	normal, happy, missing := c.Expression(ExprNormal), c.Expression("happy"), c.Expression("happy_2")
	entries := []*ImageEntry{nil, normal, happy, missing}
	PrefetchImgs(entries)
	PrefetchImgs(entries) // already on their way: nothing more to do
	if missing.pending != nil {
		t.Fatal("a portrait without a picture was queued")
	}
	deadline := time.Now().Add(30 * time.Second)
	for normal.Image == nil || happy.Image == nil {
		if time.Now().After(deadline) {
			t.Fatal("the prefetched portraits were not uploaded")
		}
		UploadPrefetched(entries, 1)
		time.Sleep(time.Millisecond)
	}
	if _, ok := KnownFigure(normal.Image); !ok {
		t.Fatal("a prefetched portrait has no figure measured")
	}
	UploadPrefetched(entries, 0)
	normal.ReleaseImg()
	happy.ReleaseImg()
}

func TestImgReadyDoesNotWait(t *testing.T) { //nolint:paralleltest // uses the prefetch queue
	c := testCharacter(t)
	e := c.Expression("happy")
	PrefetchImgs([]*ImageEntry{e})
	deadline := time.Now().Add(30 * time.Second)
	var img = e.ImgReady()
	for img == nil {
		if time.Now().After(deadline) {
			t.Fatal("the portrait never got ready")
		}
		time.Sleep(time.Millisecond)
		img = e.ImgReady()
	}
	if e.Img() != img {
		t.Fatal("Img loaded the portrait again")
	}
	e.ReleaseImg()

	// a portrait being decoded is waited for (or taken from the queue and decoded at once)
	PrefetchImgs([]*ImageEntry{e})
	if e.Img() == nil {
		t.Fatal("Img did not wait for the prefetched portrait")
	}
	e.ReleaseImg()
	PrefetchImgs([]*ImageEntry{e})
	e.ReleaseImg() // dropped while it is being decoded
	if e.pending != nil || e.Image != nil {
		t.Fatal("ReleaseImg kept a portrait being decoded")
	}
}

// TestUnqueue takes a picture out of the queue, or moves it to the front when it is wanted
// now.
func TestUnqueue(t *testing.T) { //nolint:paralleltest // uses the prefetch queue
	a, b, x := &ImageEntry{ID: "a"}, &ImageEntry{ID: "b"}, &ImageEntry{ID: "x"}
	// a worker the tests before started may still be decoding: it would take the jobs below
	// out of the queue (the test failed now and then on the full run)
	for {
		prefetchQueue.Lock()
		idle := prefetchQueue.workers == 0
		prefetchQueue.Unlock()
		if idle {
			break
		}
		time.Sleep(time.Millisecond)
	}
	prefetchQueue.Lock()
	old := prefetchQueue.jobs
	prefetchQueue.jobs = []prefetchJob{{e: a}, {e: b}}
	prefetchQueue.Unlock()
	t.Cleanup(func() {
		prefetchQueue.Lock()
		prefetchQueue.jobs = old
		prefetchQueue.Unlock()
	})
	if !unqueue(b, true) || prefetchQueue.jobs[0].e != b || prefetchQueue.jobs[1].e != a {
		t.Fatal("a picture wanted now was not moved to the front")
	}
	if !unqueue(b, false) || len(prefetchQueue.jobs) != 1 || prefetchQueue.jobs[0].e != a {
		t.Fatal("the picture was not taken out of the queue")
	}
	if unqueue(x, false) {
		t.Fatal("a picture that was never queued was found")
	}
}

func TestIllustrationsLoadAtFullSize(t *testing.T) { //nolint:paralleltest // loads the fonts
	useFonts(t)
	c := testCharacter(t)
	cg, missing := &c.CGs[0], &c.CGs[1]
	full := cg.Full()
	if full == nil || full.Bounds().Dy() != 200 || cg.Full() != full {
		t.Fatalf("the illustration loaded as %v", full)
	}
	cg.ReleaseFull()
	cg.PrefetchFull()
	cg.PrefetchFull() // on its way already
	if f := cg.Full(); f == nil || f.Bounds().Dy() != 200 {
		t.Fatalf("the prefetched illustration loaded as %v", f)
	}
	cg.PrefetchFull() // loaded already
	cg.ReleaseFull()
	missing.PrefetchFull()
	if f := missing.Full(); f == nil || f.Bounds().Dx() != 896 {
		t.Fatalf("a missing illustration gave %v, want the stand-in", f)
	}
	missing.ReleaseFull()
	if missing.Full().Bounds().Dx() != 896 {
		t.Fatal("a missing illustration loaded without a prefetch is not the stand-in")
	}
	missing.ReleaseFull()
	e := c.Expression(ExprNormal)
	if img := e.Img(); e.Full() != img {
		t.Fatal("a loaded portrait is not its own full-size picture")
	}
	e.ReleaseImg()
}

func TestDecodeScaled(t *testing.T) {
	t.Parallel()
	c := testCharacter(t)
	if img := c.Expression(ExprNormal).DecodeScaled(100); img == nil || img.Bounds().Dy() != 100 || img.Bounds().Dx() != 50 {
		t.Fatalf("scaled to 100 high: %v", img)
	}
	if img := c.Expression(ExprNormal).DecodeScaled(400); img == nil || img.Bounds().Dy() != 200 {
		t.Fatalf("a picture lower than the limit was scaled: %v", img)
	}
	if c.Expression("happy_2").DecodeScaled(100) != nil {
		t.Fatal("a missing picture decoded")
	}
}

func TestSelectEntryFallsBackToTheNormalPortrait(t *testing.T) {
	t.Parallel()
	c := testCharacter(t)
	if c.SelectEntry() != c.Select {
		t.Fatal("the select picture is not used")
	}
	c.Select = &ImageEntry{ID: "no_select"}
	if c.SelectEntry() != c.Expression(ExprNormal) {
		t.Fatal("without a select picture the normal portrait is not used")
	}
	if c.Cutin == nil || c.Cutin.fsys == nil || c.Cutin.HasImage() {
		t.Fatal("the cut-in entry is not set up from the manifest's directory")
	}
}

func TestMainAndExtraIllustrations(t *testing.T) {
	t.Parallel()
	c := &Character{}
	for i := range 30 {
		c.CGs = append(c.CGs, ImageEntry{ID: "cg" + strconv.Itoa(i)})
	}
	if len(c.MainCGs()) != MainCGCount || len(c.ExtraCGs()) != 30-MainCGCount || c.ExtraCGs()[0].ID != "cg15" {
		t.Fatalf("%d regular and %d extra illustrations", len(c.MainCGs()), len(c.ExtraCGs()))
	}
	c.CGs = c.CGs[:7]
	if len(c.MainCGs()) != 7 || len(c.ExtraCGs()) != 0 {
		t.Fatal("a character with few illustrations has extras")
	}
}

func TestMeasureFigure(t *testing.T) {
	t.Parallel()
	if f := MeasureFigure(figurePicture()); f != wantFigure {
		t.Fatalf("figure %+v, want %+v", f, wantFigure)
	}
	if f := MeasureFigure(image.NewNRGBA(image.Rect(0, 0, 10, 10))); !f.Box.Empty() {
		t.Fatalf("an empty picture has the figure %+v", f)
	}
	// other forms of picture are read through a copy
	gray := image.NewGray(image.Rect(0, 0, 4, 4))
	if f := MeasureFigure(gray); f.Box != image.Rect(0, 0, 4, 4) {
		t.Fatalf("an opaque gray picture has the figure %+v", f)
	}
	sub := figurePicture().SubImage(image.Rect(10, 0, 100, 200))
	if f := MeasureFigure(sub); f.BodyX != wantFigure.BodyX-10 {
		t.Fatalf("a cut picture has its body at %d, want %d", f.BodyX, wantFigure.BodyX-10)
	}
}

func TestPortraitBox(t *testing.T) {
	t.Parallel()
	pic := figurePicture()
	face, ok := portraitBox(pic.Pix, 100, 200, cropFace)
	if !ok || face.Min.Y < 8 || face.Min.X > 50 || face.Max.X < 50 {
		t.Fatalf("face box %v, want one around the head (not the ears)", face)
	}
	wide, ok := portraitBox(pic.Pix, 100, 200, cropWide)
	if !ok || wide.Dx() <= wide.Dy() {
		t.Fatalf("wide box %v", wide)
	}
	if _, ok := portraitBox(make([]byte, 4*10*10), 10, 10, cropFace); ok {
		t.Fatal("an empty picture has a face")
	}
}

func TestFaceOf(t *testing.T) { //nolint:paralleltest // uses the crop caches
	if FaceOf(nil) != nil {
		t.Fatal("a face of nothing")
	}
	c := testCharacter(t)
	missing := c.Expression("happy_2")
	for range 2 { // the second time from the cache
		if FaceOf(missing) != nil {
			t.Fatal("a face of a portrait without a picture")
		}
	}
	e := c.Expression(ExprNormal)
	deadline := time.Now().Add(30 * time.Second)
	face := FaceOf(e)
	for face == nil {
		if time.Now().After(deadline) {
			t.Fatal("the face close-up was never made")
		}
		time.Sleep(time.Millisecond)
		face = FaceOf(e)
	}
	if FaceOf(e) != face {
		t.Fatal("the face close-up was made twice")
	}
	if crop := cropPortrait(&ImageEntry{ID: "no_portrait", fsys: e.fsys, base: e.base}, cropFace); crop != nil {
		t.Fatal("a crop of a missing picture")
	}
}
