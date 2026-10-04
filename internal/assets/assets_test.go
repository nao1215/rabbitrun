package assets

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io/fs"
	"os"
	"testing"
	"testing/fstest"
)

// TestMain reads the game's assets directory from disk, as the game reads the embedded one.
func TestMain(m *testing.M) {
	Use(os.DirFS("../../assets"))
	os.Exit(m.Run())
}

func TestDecodeImageTriesEachExtension(t *testing.T) {
	t.Parallel()
	if img, err := Decode(FS(), "ui/hammer"); err != nil || img == nil {
		t.Fatalf("the hammer (a .png) did not decode: %v", err)
	}
	if img, err := Decode(FS(), "ui/title"); err != nil || img == nil {
		t.Fatalf("the title art (a .jpg) did not decode: %v", err)
	}
	if _, err := Decode(FS(), "ui/no_such_picture"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("a missing picture gave %v, want fs.ErrNotExist", err)
	}
	if _, err := Decode(nil, "ui/hammer"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("no file system gave %v, want fs.ErrNotExist", err)
	}
}

// TestBrokenPictureIsPassedOver: a file that does not decode is reported (not as missing),
// and the next extension is still tried.
func TestBrokenPictureIsPassedOver(t *testing.T) {
	t.Parallel()
	fsys := fstest.MapFS{"x.jpg": {Data: []byte("not a jpeg")}}
	if _, err := Decode(fsys, "x"); err == nil || errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("a broken picture gave %v, want a decoding error", err)
	}
	LogBroken("x", errors.New("broken")) // logged, not fatal
	if !HasImage(fsys, "x") || HasImage(fsys, "y") || HasImage(nil, "x") {
		t.Fatal("HasImage does not follow the files")
	}
}

func TestReadFile(t *testing.T) {
	t.Parallel()
	if raw, err := ReadFile("fonts/OFL-LilitaOne.txt"); err != nil || len(raw) == 0 {
		t.Fatalf("cannot read a file of the assets: %v", err)
	}
}

// TestUIArtwork loads a piece of artwork once (prefetched or not) and gives nil for a
// missing one.
func TestUIArtwork(t *testing.T) { //nolint:paralleltest // uses the package-level artwork cache
	PrefetchUI("hammer", "hammer")
	a := UI("hammer")
	if a == nil || UI("hammer") != a {
		t.Fatal("the hammer artwork is not loaded once")
	}
	PrefetchUI("hammer") // already loaded: nothing to do
	if UI("") != nil || UI("no_such_picture") != nil {
		t.Fatal("missing artwork must be nil")
	}
}

// TestReleaseUI forgets a piece of artwork, loaded or still decoding: the next UI reads
// it again, so a picture added under the same name is found.
func TestReleaseUI(t *testing.T) { //nolint:paralleltest // uses the package-level artwork cache
	old := FS()
	t.Cleanup(func() { Use(old); ReleaseUI("late") })
	ReleaseUI("late")
	if UI("late") != nil {
		t.Fatal("a missing picture loaded")
	}
	Use(fstest.MapFS{"ui/late.png": {Data: onePixelPNG(t)}})
	if UI("late") != nil {
		t.Fatal("the missing picture was not remembered as missing")
	}
	ReleaseUI("late")
	a := UI("late")
	if a == nil {
		t.Fatal("the picture added after a release was not read")
	}
	ReleaseUI("late") // loaded: freed
	PrefetchUI("late")
	ReleaseUI("late") // decoding: dropped
	if UI("late") == nil {
		t.Fatal("the picture was not read again after its decode was dropped")
	}
}

// TestToRGBA converts a picture to RGBA once: one already in that form comes back as it is.
func TestToRGBA(t *testing.T) {
	t.Parallel()
	src := image.NewNRGBA(image.Rect(2, 3, 6, 9))
	src.SetNRGBA(2, 3, color.NRGBA{0xff, 0, 0, 0xff})
	got, ok := ToRGBA(src).(*image.RGBA)
	if !ok || got.Bounds() != image.Rect(0, 0, 4, 6) || got.RGBAAt(0, 0) != (color.RGBA{0xff, 0, 0, 0xff}) {
		t.Fatalf("ToRGBA = %T %v", got, got.Bounds())
	}
	if ToRGBA(got) != image.Image(got) {
		t.Fatal("an RGBA picture was copied again")
	}
}

// onePixelPNG is a picture one pixel large.
func onePixelPNG(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewNRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
