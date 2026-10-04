package character

import (
	"image"
	_ "image/jpeg" // the no-miss pictures are JPEGs
	_ "image/png"
	"io/fs"
	"path"
	"slices"
	"testing"
	"testing/fstest"

	"github.com/nao1215/rabbitrun/internal/assets"
)

// noMissFS is the assets directory of one character "t", with the no-miss pictures files
// (file names under images/) beside her portrait.
func noMissFS(files ...string) fstest.MapFS {
	pic := &fstest.MapFile{Data: []byte("picture")} // only its presence is looked at
	fsys := fstest.MapFS{
		"characters/t/game.json":         {Data: []byte(`{"id":"t","order":1,"expressions":[{"id":"normal","state":"normal"}]}`)},
		"characters/t/images/normal.png": pic,
	}
	for _, f := range files {
		fsys["characters/t/images/"+f] = pic
	}
	return fsys
}

// TestNoMissPicturesOnlyWhenDrawn: a character has a no-miss picture of a side only when
// its file is there (as a JPEG or a PNG); without it the entry is nil and nothing else
// changes.
func TestNoMissPicturesOnlyWhenDrawn(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name          string
		files         []string
		regular, extr bool
	}{
		{"none", nil, false, false},
		{"regular", []string{"nomiss.jpg"}, true, false},
		{"extra", []string{"nomiss_extra.png"}, false, true},
		{"both", []string{"nomiss.png", "nomiss_extra.jpg"}, true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			chars, err := Read(noMissFS(tc.files...))
			if err != nil || len(chars) != 1 {
				t.Fatalf("read %d characters: %v", len(chars), err)
			}
			c := chars[0]
			check := func(e *ImageEntry, want bool, id string) {
				t.Helper()
				if (e != nil) != want {
					t.Fatalf("%s: entry %v, want one %v", id, e, want)
				}
				if e == nil {
					return
				}
				if e.ID != id || !e.HasImage() || e.base != "characters/t" {
					t.Errorf("%s: entry %+v", id, e)
				}
				if !slices.Contains(c.entries(), e) {
					t.Errorf("%s: not among her pictures", id)
				}
			}
			check(c.NoMiss, tc.regular, NoMissID)
			check(c.NoMissExtra, tc.extr, NoMissExtraID)
			if c.Ending == nil || c.EndingExtra == nil {
				t.Error("the endings are gone")
			}
		})
	}
}

// TestNoMissIDsAreFree: the no-miss clears are saved under their IDs among the unlocked
// illustrations, so no illustration of a character may have one of those IDs.
func TestNoMissIDsAreFree(t *testing.T) {
	t.Parallel()
	chars, err := Read(assets.FS())
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range chars {
		for _, e := range c.CGs {
			if e.ID == NoMissID || e.ID == NoMissExtraID {
				t.Errorf("%s: the illustration %q has the ID of a no-miss picture", c.ID, e.ID)
			}
		}
	}
}

// TestNoMissPicturesAreTheEndingsShape: a no-miss picture shows in the place of the
// ending, so one that is drawn is the size of the endings (896x1152).
func TestNoMissPicturesAreTheEndingsShape(t *testing.T) {
	t.Parallel()
	fsys := assets.FS()
	chars, err := Read(fsys)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range chars {
		for _, e := range []*ImageEntry{c.NoMiss, c.NoMissExtra} {
			if e == nil {
				continue
			}
			cfg, err := imageConfig(fsys, path.Join(e.base, "images", e.ID))
			if err != nil {
				t.Errorf("%s/%s: %v", c.ID, e.ID, err)
				continue
			}
			if cfg.Width != 896 || cfg.Height != 1152 {
				t.Errorf("%s/%s is %dx%d, want 896x1152", c.ID, e.ID, cfg.Width, cfg.Height)
			}
		}
	}
}

// imageConfig reads the size of the picture stem (.jpg or .png).
func imageConfig(fsys fs.FS, stem string) (image.Config, error) {
	var last error
	for _, ext := range []string{".jpg", ".png"} {
		f, err := fsys.Open(stem + ext)
		if err != nil {
			last = err
			continue
		}
		cfg, _, err := image.DecodeConfig(f)
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		return cfg, err
	}
	return image.Config{}, last
}
