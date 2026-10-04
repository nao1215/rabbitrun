package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nao1215/rabbitrun/internal/character"
	"github.com/nao1215/rabbitrun/internal/save"
)

const table = `| Character | Portraits | Illustrations |
| :---: | :---: | :---: |
| <img src="./doc/img/faces/cool.png" width="64" alt="cool"> | 1 | 15 + α (2) |
| <img src="./doc/img/faces/secret.png" width="64" alt="secret"> | 3 | 15 + α (4) |
`

func TestRewriteTable(t *testing.T) {
	t.Parallel()
	got, err := rewriteTable(table, []count{
		{face: "cool", portraits: 37, regular: 15, secret: 17},
		{face: "secret", portraits: 24, regular: 14, secret: 3},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := strings.NewReplacer(
		"| 1 | 15 + α (2) |", "| 37 | 15 + α (17) |",
		"| 3 | 15 + α (4) |", "| 24 | 14 + α (3) |",
	).Replace(table)
	if got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

func TestRewriteTableNeedsEveryRow(t *testing.T) {
	t.Parallel()
	if _, err := rewriteTable(table, []count{{face: "cute"}}); err == nil {
		t.Error("a character without a row was not reported")
	}
}

// readAssets reads the characters of the repository's assets.
func readAssets(t *testing.T) []*character.Character {
	t.Helper()
	chars, err := character.Read(os.DirFS(filepath.Join("..", "..", "assets")))
	if err != nil {
		t.Fatal(err)
	}
	if len(chars) == 0 {
		t.Fatal("no characters in assets")
	}
	return chars
}

func TestCountOfCountsWhatHasAPicture(t *testing.T) {
	t.Parallel()
	for _, c := range readAssets(t) {
		n := countOf(c)
		if n.portraits < 1 || n.portraits > len(c.Expressions) {
			t.Errorf("%s: %d portraits of %d expressions", c.ID, n.portraits, len(c.Expressions))
		}
		if n.regular > character.MainCGCount {
			t.Errorf("%s: %d regular illustrations, more than %d", c.ID, n.regular, character.MainCGCount)
		}
		if limit := len(c.ExtraCGs()) + 2; n.secret > limit {
			t.Errorf("%s: %d secret pictures, more than %d", c.ID, n.secret, limit)
		}
		if c.Secret != (n.face == "secret") {
			t.Errorf("%s: row face %q", c.ID, n.face)
		}
	}
}

func TestGallerySaveKeepsTheSecrets(t *testing.T) {
	t.Parallel()
	chars := readAssets(t)
	p := filepath.Join(t.TempDir(), "rabbitrun", "save.json")
	if err := writeGallerySave(p, chars); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(p) //nolint:gosec // G304: the file the test wrote
	if err != nil {
		t.Fatal(err)
	}
	d := &save.Data{}
	if err := save.Decode(d, raw); err != nil {
		t.Fatal(err)
	}
	if d.ExtraFound || d.WordTold {
		t.Error("the gallery save opens the extra stages")
	}
	for _, c := range chars {
		p := d.Characters[c.ID]
		if c.Secret {
			if p != nil {
				t.Errorf("the gallery save has progress for the secret character %s", c.ID)
			}
			continue
		}
		if p == nil || p.Cleared || p.ClearedExtra {
			t.Errorf("%s: progress %+v, want seen but not cleared", c.ID, p)
			continue
		}
		for _, e := range c.ExtraCGs() {
			if p.UnlockedCG[e.ID] {
				t.Errorf("%s: the extra illustration %s is unlocked", c.ID, e.ID)
			}
		}
	}
}
