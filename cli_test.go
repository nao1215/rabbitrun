package main

import (
	"errors"
	"io/fs"
	"strings"
	"testing"

	flag "github.com/spf13/pflag"

	"github.com/nao1215/rabbitrun/internal/character"
)

// parseArgs parses args into the program's own flags, as main does, and puts every
// flag back to its default when the test ends.
func parseArgs(t *testing.T, args ...string) *flag.FlagSet {
	t.Helper()
	fs := flag.CommandLine
	reset := func() {
		fs.VisitAll(func(f *flag.Flag) {
			if err := f.Value.Set(f.DefValue); err != nil {
				t.Fatal(err)
			}
			f.Changed = false
		})
	}
	reset()
	t.Cleanup(reset)
	if err := fs.Parse(args); err != nil {
		t.Fatal(err)
	}
	return fs
}

// demoArgs is a demo recording to a.mp4 with the options args.
func demoArgs(args ...string) []string {
	return append([]string{"--record-demo", "a.mp4"}, args...)
}

//nolint:paralleltest // parses into the package-level flags
func TestCheckArgs(t *testing.T) {
	const shots, capture = "shots", "--capture"
	tests := []struct {
		name string
		args []string
		want string // a part of the error; empty when the command line is fine
	}{
		{"no options start the game", nil, ""},
		{"one exit mode", []string{capture, shots}, ""},
		{"a demo with its options", demoArgs("--record-char", "cute", "--record-stage", "4", "--record-seconds", "1"), ""},
		{"debug goes with an exit mode", []string{"--debug", capture, shots}, ""},
		{"a forgotten option name", []string{shots}, `unexpected argument "shots"`},
		{"an empty directory", []string{"--capture="}, "--capture needs a non-empty value"},
		{"an empty video path", []string{"--record-demo="}, "--record-demo needs a non-empty value"},
		{"two exit modes", []string{"--bgm-wav", "w", "--reset-save"}, "--bgm-wav and --reset-save cannot be used together"},
		{"a demo option without a demo", []string{"--record-stage", "2"}, "--record-stage only works with --record-demo"},
		{"a demo character without a demo", []string{"--record-char", "cute"}, "--record-char only works with --record-demo"},
		{"stage 0", demoArgs("--record-stage", "0"), "--record-stage must be from 1 to 4, got 0"},
		{"a stage past the last", demoArgs("--record-stage", "5"), "--record-stage must be from 1 to 4, got 5"},
		{"no seconds", demoArgs("--record-seconds", "0"), "--record-seconds must be 1 or more, got 0"},
		{"negative seconds", demoArgs("--record-seconds=-3"), "--record-seconds must be 1 or more, got -3"},
		// a switch turned off is not asked for: it does nothing, so it cannot clash
		{"reset turned off goes with an exit mode", []string{"--reset-save=false", capture, shots}, ""},
		{"extra turned off without a demo", []string{"--record-extra=false"}, ""},
		{"reset turned on still clashes", []string{"--reset-save=true", capture, shots}, "--capture and --reset-save cannot be used together"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkArgs(parseArgs(t, tt.args...))
			if tt.want == "" {
				if err != nil {
					t.Fatalf("checkArgs(%q) = %v, want no error", tt.args, err)
				}
				return
			}
			var ue *usageError
			if !errors.As(err, &ue) {
				t.Fatalf("checkArgs(%q) = %v, want a usage error", tt.args, err)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("checkArgs(%q) = %q, want it to contain %q", tt.args, err, tt.want)
			}
		})
	}
}

func TestDemoStagesMatchTheGame(t *testing.T) {
	t.Parallel()
	if demoStages != 4 {
		t.Errorf("demoStages = %d, want 4 (four stages of four courses)", demoStages)
	}
}

func TestCheckRecordChar(t *testing.T) {
	t.Parallel()
	ids := []string{"cool", "gyal", "bunny"}
	if err := checkRecordChar("", ids); err != nil {
		t.Errorf("no character should pick the main one, got %v", err)
	}
	if err := checkRecordChar("gyal", ids); err != nil {
		t.Errorf("a character of the game should be accepted, got %v", err)
	}
	err := checkRecordChar("nobody", ids)
	var ue *usageError
	if !errors.As(err, &ue) {
		t.Fatalf("an unknown character should be a usage error, got %v", err)
	}
	for _, want := range []string{`unknown character "nobody"`, "cool, gyal, bunny"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q should contain %q", err, want)
		}
	}
}

// TestEmbeddedAssetsHoldTheGame: go:embed takes the game data from the module root, so
// the embedded directory must hold what the game reads: the fonts, the artwork, the
// blocks and the characters with their pictures.
func TestEmbeddedAssetsHoldTheGame(t *testing.T) {
	t.Parallel()
	fsys := embeddedAssets()
	for _, name := range []string{"fonts/mplus-1p-regular.ttf", "fonts/LilitaOne-Regular.ttf", "ui/title.jpg", "blocks/soda.png"} {
		if _, err := fs.Stat(fsys, name); err != nil {
			t.Errorf("%s is not embedded: %v", name, err)
		}
	}
	chars, err := character.Read(fsys)
	if err != nil || len(chars) == 0 {
		t.Fatalf("no characters embedded (%v)", err)
	}
	for _, c := range chars {
		if !c.Expression(character.ExprNormal).HasImage() {
			t.Errorf("%s: her standing picture is not embedded", c.ID)
		}
	}
	if _, err := fs.Stat(fsys, "characters/"+chars[0].ID+"/character.json"); err == nil {
		t.Error("a character's private notes are embedded; only game.json and the images ship")
	}
}

// TestFindFFmpegTellsWhatIsMissing runs with an empty PATH.
func TestFindFFmpegTellsWhatIsMissing(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	err := findFFmpeg()
	if err == nil || !strings.Contains(err.Error(), "--record-demo needs ffmpeg on PATH") {
		t.Fatalf("findFFmpeg() = %v, want an error naming ffmpeg", err)
	}
}
