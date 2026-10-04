package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"slices"
	"strings"

	flag "github.com/spf13/pflag"

	"github.com/nao1215/rabbitrun/internal/engine"
	"github.com/nao1215/rabbitrun/road"
)

// The checks on the command line that run before anything is loaded: a mistake in how
// the program was started is told at once, instead of the game opening anyway with the
// option ignored.

// usageError is a mistake in the command line; main reports it with exit status 2, the
// status of a wrong option.
type usageError struct{ msg string }

func (e *usageError) Error() string { return e.msg }

// usageErrorf returns a command-line mistake described by format.
func usageErrorf(format string, a ...any) error {
	return &usageError{msg: fmt.Sprintf(format, a...)}
}

// exitUsage reports the command-line mistake err with a pointer to the help and exits
// with status 2.
func exitUsage(err error) {
	if _, werr := fmt.Fprintf(os.Stderr, "rabbitrun: %v\nRun 'rabbitrun --help' to see the options.\n", err); werr != nil {
		log.Print(werr)
	}
	os.Exit(2)
}

// exitModes are the options that do one job and exit instead of starting the game.
var exitModes = []string{"bgm-wav", "capture", "reset-save", "record-demo"}

// recordOptions only change the demo recording.
var recordOptions = []string{"record-char", "record-stage", "record-seconds"}

// demoStages is how many stages a demo recording can start at.
const demoStages = (engine.GameCourses + road.Courses - 1) / road.Courses

// checkArgs checks the parsed command line fs for options the game would otherwise
// ignore or misread.
func checkArgs(fs *flag.FlagSet) error {
	if fs.NArg() > 0 {
		return usageErrorf("unexpected argument %q (options start with --, e.g. --capture DIR)", fs.Arg(0))
	}
	var modes []string
	for _, name := range exitModes {
		if !fs.Changed(name) {
			continue
		}
		modes = append(modes, "--"+name)
		if v := fs.Lookup(name).Value; v.Type() == "string" && v.String() == "" {
			return usageErrorf("--%s needs a non-empty value", name)
		}
	}
	if len(modes) > 1 {
		return usageErrorf("%s cannot be used together; run them one at a time", strings.Join(modes, " and "))
	}
	if !fs.Changed("record-demo") {
		for _, name := range recordOptions {
			if fs.Changed(name) {
				return usageErrorf("--%s only works with --record-demo FILE", name)
			}
		}
		return nil
	}
	if *recordStage < 1 || *recordStage > demoStages {
		return usageErrorf("--record-stage must be from 1 to %d, got %d", demoStages, *recordStage)
	}
	if *recordSeconds < 1 {
		return usageErrorf("--record-seconds must be 1 or more, got %d", *recordSeconds)
	}
	return nil
}

// checkRecordChar checks that id (from --record-char) is one of the character IDs ids; an
// empty id picks the main character.
func checkRecordChar(id string, ids []string) error {
	if id == "" || slices.Contains(ids, id) {
		return nil
	}
	return usageErrorf("unknown character %q for --record-char (choose from %s)", id, strings.Join(ids, ", "))
}

// findFFmpeg checks that ffmpeg, which --record-demo encodes the video with, is on PATH.
func findFFmpeg() error {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return fmt.Errorf("--record-demo needs ffmpeg on PATH to encode the video: %w", err)
	}
	return nil
}
