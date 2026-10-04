package character

import (
	"image"
	"image/color"
	"io/fs"
	"path"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/draw"

	"github.com/nao1215/rabbitrun/internal/assets"
	"github.com/nao1215/rabbitrun/internal/gfx"
)

// ImageEntry is a picture of a character: a portrait, an illustration, or one of her
// other pictures (select, group, cut-in, ending). It is loaded on use.
type ImageEntry struct {
	ID    string `json:"id"`
	State string `json:"state"` // state this portrait is for (a state can have several poses)
	Label string `json:"label"`
	Title string `json:"title"`
	// Order sorts the illustrations (the order they unlock in). The key is still "score",
	// from the old scored game where they unlocked at a score, so game.json files load.
	Order int `json:"score"` //nolint:tagliatelle // key used by the existing game.json files

	Image *ebiten.Image `json:"-"` // portraits load lazily via Img; illustrations load on demand via Full

	fsys        fs.FS  // the assets directory the picture is read from (Read)
	base        string // the character's directory in fsys
	full        *ebiten.Image
	pending     chan decodedImg  // the portrait being decoded in the background (PrefetchImgs)
	fullPending chan image.Image // the illustration being decoded in the background (PrefetchFull)
	has         int8             // HasImage cache: 0 unknown, 1 yes, -1 no (the assets are embedded and never change)
}

// HasImage reports whether the image file exists (otherwise a placeholder is used).
func (e *ImageEntry) HasImage() bool {
	if e.has == 0 {
		e.has = -1
		if assets.HasImage(e.fsys, path.Join(e.base, "images", e.ID)) {
			e.has = 1
		}
	}
	return e.has > 0
}

// standingMaxH caps the height of loaded portraits. Keeping 156 of them at full size (640x1600) uses too much memory, so they are scaled down.
const standingMaxH = 800

// Img loads the portrait (expression or character-select image) scaled down on first use.
// If the picture is being decoded in the background (PrefetchImgs), it waits for that
// rather than decoding it twice (unless it has not started yet).
func (e *ImageEntry) Img() *ebiten.Image {
	if e.Image == nil {
		if e.pending != nil && !unqueue(e, false) {
			e.upload(<-e.pending)
		} else {
			e.pending = nil
			e.upload(decodePortrait(e.fsys, e.base, e.ID))
		}
	}
	return e.Image
}

// ImgReady is Img without the wait: nil while the picture is still being decoded in the
// background.
func (e *ImageEntry) ImgReady() *ebiten.Image {
	if e.Image == nil && e.pending != nil {
		select {
		case d := <-e.pending:
			e.upload(d)
		default:
			unqueue(e, true) // wanted now: decoded next
			return nil
		}
	}
	return e.Img()
}

// name is what the placeholder of a missing picture says.
func (e *ImageEntry) name() string {
	if e.Label != "" {
		return e.Label
	}
	return e.ID
}

// upload hands a picture decoded in the background to the GPU, with its figure measured.
func (e *ImageEntry) upload(d decodedImg) {
	e.pending = nil
	if d.img == nil {
		e.Image = Placeholder(e.name())
		return
	}
	e.Image = ebiten.NewImageFromImage(d.img)
	figureCache[e.Image] = d.fig
}

// ReleaseImg frees the loaded portrait on the GPU (and drops one being decoded); Img
// loads it again when it is wanted.
func (e *ImageEntry) ReleaseImg() {
	if e.pending != nil {
		unqueue(e, false)
		e.pending = nil
	}
	if e.Image != nil {
		delete(figureCache, e.Image)
		e.Image.Deallocate()
		e.Image = nil
	}
}

// decodedImg is a picture decoded off the main goroutine, with its figure measured there
// too (so it is not read back from the GPU).
type decodedImg struct {
	img image.Image
	fig Figure
}

// decodePortrait decodes images/<id> as Img shows it (at most standingMaxH high) and
// measures its figure on the CPU, so drawing it need not read it back from the GPU. It
// may run on any goroutine.
func decodePortrait(fsys fs.FS, base, id string) decodedImg {
	var d decodedImg
	if d.img = decodeScaled(fsys, base, id, standingMaxH); d.img != nil {
		d.fig = MeasureFigure(d.img)
	}
	return d
}

// decoders limits how many pictures are decoded at the same time. A few are enough: the
// pictures are uploaded only a few a frame anyway, and every decoder holds a portrait of
// several megabytes while it works (one per CPU spiked memory to hundreds of MB).
var decoders = make(chan struct{}, min(3, max(1, runtime.NumCPU()-1)))

// PrefetchImgs decodes the pictures of entries, as Img loads them, in the background: in
// the order given, a few at a time. Img then only uploads them (and UploadPrefetched does
// it a few a frame ahead of use). A picture already loaded or on its way is left alone.
func PrefetchImgs(entries []*ImageEntry) {
	prefetchQueue.Lock()
	defer prefetchQueue.Unlock()
	for _, e := range entries {
		if e == nil || e.Image != nil || e.pending != nil || !e.HasImage() {
			continue
		}
		ch := make(chan decodedImg, 1)
		e.pending = ch
		prefetchQueue.jobs = append(prefetchQueue.jobs, prefetchJob{e, e.fsys, e.base, e.ID, ch})
	}
	for ; prefetchQueue.workers < min(len(prefetchQueue.jobs), cap(decoders)); prefetchQueue.workers++ {
		go prefetchWorker()
	}
}

// prefetchWaiting counts the prefetch workers waiting for a decoder; the gallery's tile
// decodes (DecodeScaled) step aside while it is not zero.
var prefetchWaiting atomic.Int32

// prefetchQueue holds the pictures waiting to be decoded in the background, and how many
// workers are decoding them.
var prefetchQueue struct {
	sync.Mutex
	jobs    []prefetchJob
	workers int
}

// prefetchJob is a picture to decode. The worker reads only fsys, base and id; e only
// tells the jobs apart.
type prefetchJob struct {
	e        *ImageEntry
	fsys     fs.FS
	base, id string
	out      chan<- decodedImg
}

// prefetchWorker decodes queued pictures until the queue is empty.
func prefetchWorker() {
	for {
		prefetchQueue.Lock()
		if len(prefetchQueue.jobs) == 0 {
			prefetchQueue.workers--
			prefetchQueue.Unlock()
			return
		}
		j := prefetchQueue.jobs[0]
		prefetchQueue.jobs = prefetchQueue.jobs[1:]
		prefetchQueue.Unlock()
		prefetchWaiting.Add(1)
		decoders <- struct{}{}
		prefetchWaiting.Add(-1)
		d := decodePortrait(j.fsys, j.base, j.id)
		<-decoders
		j.out <- d
	}
}

// unqueue takes e's picture out of the queue (keep: put it first instead, as it is wanted
// now). It reports whether it was still waiting there.
func unqueue(e *ImageEntry, keep bool) bool {
	prefetchQueue.Lock()
	defer prefetchQueue.Unlock()
	q := prefetchQueue.jobs
	for i, j := range q {
		if j.e == e {
			copy(q[1:i+1], q[:i]) // the jobs before it move back one
			if !keep {
				prefetchQueue.jobs = q[1:]
				return true
			}
			q[0] = j
			return true
		}
	}
	return false
}

// UploadPrefetched uploads up to n of the entries' pictures that finished decoding
// (uploading is quick, but capped so a frame never stalls).
func UploadPrefetched(entries []*ImageEntry, n int) {
	for _, e := range entries {
		if n == 0 {
			return
		}
		if e == nil || e.Image != nil || e.pending == nil {
			continue
		}
		select {
		case d := <-e.pending:
			e.upload(d)
			n--
		default:
		}
	}
}

// DecodeScaled decodes the picture at most maxH pixels high, or returns nil if it is
// missing. It may run on any goroutine, and waits while a few other pictures are being
// decoded (see decoders). Closing quit gives up the wait (it returns nil): the screen that
// wanted the picture was left, and the decoders are wanted for the next one's pictures.
func (e *ImageEntry) DecodeScaled(maxH int, quit <-chan struct{}) image.Image {
	// The gallery's tiles come after the pictures wanted now (the prefetch queue): with one
	// decoder (two CPUs) the enlarged view waited behind dozens of tiles.
	for {
		if prefetchWaiting.Load() == 0 {
			select {
			case decoders <- struct{}{}:
			case <-quit:
				return nil
			case <-time.After(2 * time.Millisecond):
				continue // look again whether a prefetch is waiting
			}
			if prefetchWaiting.Load() == 0 {
				break
			}
			<-decoders // a prefetch came meanwhile: let it go first
			continue
		}
		select {
		case <-quit:
			return nil
		case <-time.After(2 * time.Millisecond):
		}
	}
	defer func() { <-decoders }()
	return decodeScaled(e.fsys, e.base, e.ID, maxH)
}

// Full loads the illustration at full size (for the gallery's enlarged view). Free it with
// ReleaseFull. If PrefetchFull started decoding it, it only waits for that and uploads it.
func (e *ImageEntry) Full() *ebiten.Image {
	if e.Image != nil {
		return e.Image
	}
	if e.full == nil {
		if e.fullPending != nil {
			e.uploadFull(<-e.fullPending)
		} else {
			e.full = loadCharImage(e.fsys, e.base, e.ID, e.Title)
		}
	}
	return e.full
}

// uploadFull hands the illustration decoded in the background to the GPU (a placeholder
// if it is missing).
func (e *ImageEntry) uploadFull(img image.Image) {
	e.fullPending = nil
	if img != nil {
		e.full = ebiten.NewImageFromImage(img)
	} else {
		e.full = Placeholder(e.Title)
	}
}

// FullReady is Full without the wait: it starts decoding the illustration in the
// background if nothing has (PrefetchFull), and is nil until it is decoded. The gallery's
// enlarged view decoded each picture on the main goroutine, a frame of 40 to 80 ms at
// every step.
func (e *ImageEntry) FullReady() *ebiten.Image {
	if e.Image != nil || e.full != nil {
		return e.Full()
	}
	e.PrefetchFull()
	select {
	case img := <-e.fullPending:
		e.uploadFull(img)
		return e.full
	default:
		return nil
	}
}

// PrefetchFull starts decoding the full-size illustration in the background, so the frame
// that first shows it (Full) only uploads it: decoding a large JPEG on the main goroutine
// dropped frames whenever the road changed its picture.
func (e *ImageEntry) PrefetchFull() {
	if e.Image != nil || e.full != nil || e.fullPending != nil {
		return
	}
	ch := make(chan image.Image, 1)
	e.fullPending = ch
	fsys, base, id := e.fsys, e.base, e.ID
	go func() { ch <- decodeCharImage(fsys, base, id) }()
}

// ReleaseFull frees the full-size illustration on the GPU (and drops one being decoded).
func (e *ImageEntry) ReleaseFull() {
	e.fullPending = nil
	if e.full != nil {
		e.full.Deallocate()
		e.full = nil
	}
}

// decodeScaled reads images/<id> and scales it down to at most maxH pixels high. It only
// touches the CPU, so it may run on any goroutine. It returns nil if the image is missing.
func decodeScaled(fsys fs.FS, base, id string, maxH int) image.Image {
	img, err := assets.Decode(fsys, path.Join(base, "images", id))
	if err != nil {
		assets.LogBroken(id, err)
		return nil
	}
	b := img.Bounds()
	if b.Dy() > maxH {
		w := b.Dx() * maxH / b.Dy()
		dst := image.NewRGBA(image.Rect(0, 0, w, maxH))
		draw.ApproxBiLinear.Scale(dst, dst.Bounds(), img, b, draw.Src, nil)
		return dst
	}
	return assets.ToRGBA(img)
}

// loadCharImage reads images/<id>.jpg (or .png). If missing, it returns a placeholder.
func loadCharImage(fsys fs.FS, base, id, label string) *ebiten.Image {
	if img := decodeCharImage(fsys, base, id); img != nil {
		return ebiten.NewImageFromImage(img)
	}
	return Placeholder(label)
}

// decodeCharImage decodes images/<id>.jpg (or .png) at full size, or returns nil if it is
// missing. It only touches the CPU, so it may run on any goroutine.
func decodeCharImage(fsys fs.FS, base, id string) image.Image {
	img, err := assets.Decode(fsys, path.Join(base, "images", id))
	if err != nil {
		assets.LogBroken(id, err)
		return nil
	}
	return assets.ToRGBA(img)
}

// Placeholder is the stand-in (a human silhouette saying label) used when an image is
// missing.
func Placeholder(label string) *ebiten.Image {
	img := ebiten.NewImage(896, 1120)
	sil := color.NRGBA{0xe0, 0xa8, 0xb8, 0xd0}
	vector.FillCircle(img, 448, 300, 130, sil, true)
	gfx.FillRoundRect(img, 268, 450, 360, 670, 120, sil)
	gfx.DrawText(img, label, 448, 700, 56, color.White)
	return img
}
