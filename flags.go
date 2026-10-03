package main

import (
	flag "github.com/spf13/pflag"
)

// The command-line options. The game is played without any: they are for checking it
// (screenshots, the music, a demo video) and for starting over.

// showHelp: -h / --help prints the usage and exits.
var showHelp = flag.BoolP("help", "h", false, "show this help and exit")

// version is the release version, set at build time with -ldflags "-X main.version=vX.Y.Z".
var version = "dev"

// showVersion: with -version, the version is printed and the game exits.
var showVersion = flag.BoolP("version", "V", false, "print the version and exit")

// captureDir: with -capture <dir>, each screen is saved as a PNG and the game exits (for visual checks).
var captureDir = flag.String("capture", "", "save a screenshot of every screen to `DIR` and exit")

// debugMode: with --debug, the whole gallery is unlocked (for checking images; save data is unchanged).
var debugMode = flag.Bool("debug", false, "unlock every character, portrait and illustration for this run")

// resetSaveFlag: with --reset-save, the save data is moved aside before the game starts.
var resetSaveFlag = flag.Bool("reset-save", false, "delete the save data (kept as save.json.bak) and start from the beginning")

// bgmWavDir: with -bgm-wav <dir>, 30 seconds of each character's BGM arrangement is written as WAV and the game exits (for listening).
var bgmWavDir = flag.String("bgm-wav", "", "write each music arrangement to `DIR` as WAV and exit")

// recordPath: with --record-demo <file.mp4>, the game plays itself and the frames are
// piped to ffmpeg to make a video of the rules in motion, then it exits.
var recordPath = flag.String("record-demo", "", "play a demo by itself and save it as a video to `FILE` (needs ffmpeg)")

// recordChar picks the character of the demo recording (by ID; the main character by default).
var recordChar = flag.String("record-char", "", "character `ID` of the demo recording")

// recordStage starts the demo recording at a later stage (1 for the first).
var recordStage = flag.Int("record-stage", 1, "stage the demo recording starts at")

// recordSeconds is how long the demo recording lasts.
var recordSeconds = flag.Int("record-seconds", 60, "length of the demo recording in seconds")
