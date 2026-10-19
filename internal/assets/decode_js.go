//go:build js

package assets

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"io"
	"syscall/js"
)

// In a browser the game and everything it runs share one thread: a portrait decoded in Go
// (WebP with alpha) held it for 40 ms on a desktop and several times that on a phone, so
// the screen stalled and the music, starved of its buffer, crackled whenever new pictures
// came in. The browser decodes pictures on threads of its own (createImageBitmap), and
// the waiting lets the game go on; only reading the pixels back takes the game's thread.
// Where that is missing (Node, an old browser), Go decodes as on the desktop.

// decodeImage decodes the picture read from r, by the browser when it can.
func decodeImage(r io.Reader) (image.Image, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	if img, err := browserDecode(raw); err == nil {
		return img, nil
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	return img, err
}

// errNoBrowserDecoder is returned where the page cannot decode pictures for Go.
var errNoBrowserDecoder = errors.New("no createImageBitmap or canvas")

// scratchCanvas is the canvas the decoded pictures are drawn on to read their pixels.
var scratchCanvas, scratchCtx js.Value

// await waits for the promise p to settle (other goroutines run meanwhile).
func await(p js.Value) (js.Value, error) {
	type result struct {
		v   js.Value
		err error
	}
	ch := make(chan result, 1)
	ok := js.FuncOf(func(_ js.Value, args []js.Value) any {
		ch <- result{v: args[0]}
		return nil
	})
	fail := js.FuncOf(func(_ js.Value, args []js.Value) any {
		ch <- result{err: fmt.Errorf("%s", args[0].Call("toString").String())}
		return nil
	})
	defer ok.Release()
	defer fail.Release()
	p.Call("then", ok, fail)
	r := <-ch
	return r.v, r.err
}

// browserDecode decodes raw with the browser's decoders, as premultiplied RGBA.
func browserDecode(raw []byte) (img image.Image, err error) {
	defer func() {
		if r := recover(); r != nil { // a call that throws (syscall/js panics)
			img, err = nil, fmt.Errorf("decoding in the browser: %v", r)
		}
	}()
	global := js.Global()
	if global.Get("createImageBitmap").Type() != js.TypeFunction {
		return nil, errNoBrowserDecoder
	}
	if scratchCtx.IsUndefined() {
		switch {
		case global.Get("OffscreenCanvas").Type() == js.TypeFunction:
			scratchCanvas = global.Get("OffscreenCanvas").New(1, 1)
		case global.Get("document").Type() == js.TypeObject:
			scratchCanvas = global.Get("document").Call("createElement", "canvas")
		default:
			return nil, errNoBrowserDecoder
		}
		scratchCtx = scratchCanvas.Call("getContext", "2d", map[string]any{"willReadFrequently": true})
		if scratchCtx.IsNull() || scratchCtx.IsUndefined() {
			scratchCtx = js.Undefined()
			return nil, errNoBrowserDecoder
		}
	}
	u8 := global.Get("Uint8Array").New(len(raw))
	js.CopyBytesToJS(u8, raw)
	blob := global.Get("Blob").New([]any{u8})
	bmp, err := await(global.Call("createImageBitmap", blob, map[string]any{
		"premultiplyAlpha": "none", "colorSpaceConversion": "none",
	}))
	if err != nil {
		return nil, err
	}
	defer bmp.Call("close")
	w, h := bmp.Get("width").Int(), bmp.Get("height").Int()
	// Nothing yields between drawing and reading back, so one canvas serves every decode.
	scratchCanvas.Set("width", w)
	scratchCanvas.Set("height", h)
	scratchCtx.Call("clearRect", 0, 0, w, h)
	scratchCtx.Call("drawImage", bmp, 0, 0)
	data := scratchCtx.Call("getImageData", 0, 0, w, h).Get("data")
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	js.CopyBytesToGo(dst.Pix, global.Get("Uint8Array").New(data.Get("buffer"), data.Get("byteOffset"), data.Get("byteLength")))
	premultiply(dst.Pix)
	scratchCanvas.Set("width", 1) // let the canvas's memory go
	scratchCanvas.Set("height", 1)
	return dst, nil
}

// premultiply turns the straight-alpha RGBA the canvas gives into the premultiplied form
// image.RGBA holds, as image/draw converts it.
func premultiply(pix []byte) {
	for i := 0; i+3 < len(pix); i += 4 {
		switch a := uint32(pix[i+3]); a {
		case 0xff:
		case 0:
			pix[i], pix[i+1], pix[i+2] = 0, 0, 0
		default:
			a |= a << 8
			for j := i; j < i+3; j++ {
				c := uint32(pix[j]) | uint32(pix[j])<<8
				pix[j] = uint8(c * a / 0xffff >> 8) //nolint:gosec // G115: at most 0xffff>>8, a byte
			}
		}
	}
}
