package assets

import (
	"image"
	"io/fs"

	"github.com/hajimehoshi/ebiten/v2"
)

// The artwork of the screens (ui/): loaded once, kept on the GPU, and decoded ahead in the
// background where a screen knows what it will show.

var uiCache = map[string]*ebiten.Image{}

// UI returns the artwork ui/<name>.jpg (or .png), or nil if it is missing. If PrefetchUI
// started decoding it, it only waits for that and uploads it.
func UI(name string) *ebiten.Image {
	if name == "" {
		return nil
	}
	if img, ok := uiCache[name]; ok {
		return img
	}
	var d decodedUI
	if ch, ok := uiPending[name]; ok {
		d = <-ch
		delete(uiPending, name)
	} else {
		d = decodeUI(root, name)
	}
	var img *ebiten.Image
	if d.err == nil {
		img = ebiten.NewImageFromImage(d.img)
	} else {
		LogBroken(name, d.err)
	}
	uiCache[name] = img
	return img
}

// decodedUI is a piece of artwork decoded off the main goroutine.
type decodedUI struct {
	img image.Image
	err error
}

// decodeUI decodes ui/<name> of fsys as UI uploads it. It may run on any goroutine, so it
// takes the assets directory instead of reading root, which Use may change meanwhile.
func decodeUI(fsys fs.FS, name string) decodedUI {
	img, err := Decode(fsys, "ui/"+name)
	if err != nil {
		return decodedUI{err: err}
	}
	return decodedUI{img: ToRGBA(img)}
}

// uiPending holds the artwork PrefetchUI is decoding in the background. Only the main
// goroutine uses the map; each decoder only sends on its channel.
var uiPending = map[string]chan decodedUI{}

// PrefetchUI starts decoding the artwork names in the background, so the frame that first
// shows one only uploads it: a frame of the play screen decoded the frame of a new
// expression (a JPEG) on the main goroutine, 5 to 6 ms, the first time it showed.
func PrefetchUI(names ...string) {
	for _, name := range names {
		if _, ok := uiCache[name]; ok {
			continue
		}
		if _, ok := uiPending[name]; ok {
			continue
		}
		ch := make(chan decodedUI, 1)
		uiPending[name] = ch
		fsys := root
		go func() { ch <- decodeUI(fsys, name) }()
	}
}
