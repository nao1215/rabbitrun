// Command readme keeps the README's picture counts in step with the assets, and writes
// the save data the gallery screenshot is taken with. scripts/readme_assets.sh (make
// readme) runs it; it reads the characters the same way the game does, so the numbers
// are the pictures a player can collect.
//
//	go run ./scripts/readme counts [-assets DIR] [-readme FILE]
//	go run ./scripts/readme gallery-save [-assets DIR] FILE
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"

	"github.com/nao1215/rabbitrun/internal/character"
	"github.com/nao1215/rabbitrun/internal/save"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		if _, werr := fmt.Fprintf(os.Stderr, "readme: %v\n", err); werr != nil {
			os.Exit(2)
		}
		os.Exit(1)
	}
}

// run runs the subcommand in args, reporting what it did to w.
func run(args []string, w io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: readme counts [-assets DIR] [-readme FILE] | readme gallery-save [-assets DIR] FILE")
	}
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	assetsDir := fs.String("assets", "assets", "the assets directory")
	readmePath := fs.String("readme", "README.md", "the README to update (counts)")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	chars, err := character.Read(os.DirFS(*assetsDir))
	if err != nil {
		return err
	}
	switch args[0] {
	case "counts":
		return updateCounts(*readmePath, chars, w)
	case "gallery-save":
		if fs.NArg() != 1 {
			return errors.New("gallery-save needs the path of the save file to write")
		}
		return writeGallerySave(fs.Arg(0), chars)
	default:
		return fmt.Errorf("unknown subcommand %q (counts or gallery-save)", args[0])
	}
}

// count is what the README's character table says about one character.
type count struct {
	face      string // the face image of her row (doc/img/faces/<face>.png)
	portraits int    // the standing portraits the gallery lists
	regular   int    // the illustrations of the regular stages
	secret    int    // the extra illustrations and the two endings (the α)
}

// countOf counts the pictures of c that have an image, as the gallery lists them: the
// expressions are the portraits; the first character.MainCGCount illustrations are the
// regular ones, and the extra ones plus the two endings are the secret part. The secret
// character's row keeps her silhouette face.
func countOf(c *character.Character) count {
	n := count{face: c.ID}
	if c.Secret {
		n.face = "secret"
	}
	for i := range c.Expressions {
		if c.Expressions[i].HasImage() {
			n.portraits++
		}
	}
	main, extra := c.MainCGs(), c.ExtraCGs()
	for i := range main {
		if main[i].HasImage() {
			n.regular++
		}
	}
	for i := range extra {
		if extra[i].HasImage() {
			n.secret++
		}
	}
	for _, e := range []*character.ImageEntry{c.Ending, c.EndingExtra} {
		if e != nil && e.HasImage() {
			n.secret++
		}
	}
	return n
}

// rowRe matches a row of the character table: the face cell, the portraits and the
// illustrations.
var rowRe = regexp.MustCompile(`(?m)^(\| <img src="\./doc/img/faces/([a-z0-9_]+)\.png"[^|]*\|) [^|]* \| [^|]* \|$`)

// rewriteTable puts counts into the character table of readme. Every character must have
// a row; the rows, their order and their faces stay as they are.
func rewriteTable(readme string, counts []count) (string, error) {
	byFace := make(map[string]count, len(counts))
	for _, n := range counts {
		byFace[n.face] = n
	}
	seen := map[string]bool{}
	out := rowRe.ReplaceAllStringFunc(readme, func(row string) string {
		m := rowRe.FindStringSubmatch(row)
		n, ok := byFace[m[2]]
		if !ok {
			return row
		}
		seen[m[2]] = true
		return fmt.Sprintf("%s %d | %d + α (%d) |", m[1], n.portraits, n.regular, n.secret)
	})
	for _, n := range counts {
		if !seen[n.face] {
			return "", fmt.Errorf("the README has no row for doc/img/faces/%s.png", n.face)
		}
	}
	return out, nil
}

// updateCounts rewrites the character table of the README at path.
func updateCounts(path string, chars []*character.Character, w io.Writer) error {
	counts := make([]count, 0, len(chars))
	for _, c := range chars {
		n := countOf(c)
		counts = append(counts, n)
		if _, err := fmt.Fprintf(w, "%-8s portraits %3d  illustrations %d + α (%d)\n", c.ID, n.portraits, n.regular, n.secret); err != nil {
			return err
		}
	}
	raw, err := os.ReadFile(path) //nolint:gosec // G304: the README the caller names
	if err != nil {
		return err
	}
	out, err := rewriteTable(string(raw), counts)
	if err != nil {
		return err
	}
	if out == string(raw) {
		return nil
	}
	return os.WriteFile(path, []byte(out), 0o600) //nolint:gosec // G703: the README the caller names
}

// gallerySave is the save data of the gallery screenshot: every regular character has
// seen all her portraits and unlocked the regular illustrations, but none has cleared the
// game, so the secret character stays a silhouette and the extra illustrations unlisted.
func gallerySave(chars []*character.Character) *save.Data {
	d := &save.Data{Characters: map[string]*save.CharProgress{}}
	for _, c := range chars {
		if c.Secret {
			continue
		}
		p := d.Progress(c.ID)
		for i := range c.Expressions {
			p.SeenExpr[c.Expressions[i].ID] = true
		}
		main := c.MainCGs()
		for i := range main {
			p.UnlockedCG[main[i].ID] = true
		}
	}
	return d
}

// writeGallerySave writes the save data of the gallery screenshot to path.
func writeGallerySave(path string, chars []*character.Character) error {
	raw, err := json.MarshalIndent(gallerySave(chars), "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil { //nolint:gosec // G703: the save file the caller names
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o600) //nolint:gosec // G703: the save file the caller names
}
