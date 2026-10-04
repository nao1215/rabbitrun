## Contributing to rabbitrun

Thank you for helping make rabbitrun better. Bug reports, ideas, patches, tests and reviews are all welcome.

### 1. Start with clear communication

- Bug report: use the issue template and include the steps to reproduce, what you expected and what happened. A screenshot helps a lot.
- New feature: open an issue first so we can agree on the direction before you implement it.
- Bug fix or improvement: open a pull request with a clear problem statement and a summary of the fix.

### 2. Build and run

You need Go 1.26.6 or later. No C compiler is needed; on Linux, install the runtime libraries listed in the [Ebitengine install guide](https://ebitengine.org/en/documents/install.html) to run the game.

```shell
make build   # build ./rabbitrun
make run     # build and start the game
```

Useful flags while developing:

- `./rabbitrun --debug` unlocks every character and picture for one run.
- `./rabbitrun --capture DIR` walks through every screen, saves a screenshot of each and exits.
- `./rabbitrun --bgm-wav DIR` renders each background-music arrangement to WAV and exits.
- `./rabbitrun --record-demo FILE` lets the game play by itself and records it with ffmpeg (`--record-char`, `--record-stage` and `--record-seconds` pick what is recorded).

The screenshots and the demo GIF in the README come from `--capture` and `--record-demo`.

### Where the code lives

The `main` package at the repository root only reads the command line and opens the window (`main.go`, `flags.go`, `cli.go`, `usage.go`); it also embeds `assets/` (`assets.go`), because `go:embed` only reaches files under the package's own directory. The game itself is in these packages:

- `internal/game`: the screens (title, character select, play, the gallery), the background and the gummy blocks, and the scripted runs of `--capture` and `--record-demo`.
- `road`: the rules of the road (the wandering road, sweets, crashes, lives).
- `internal/engine`: the road in time (scroll and slide speeds), each character's run of courses, and the auto player.
- `internal/character`: the characters' manifests (`assets/characters/<id>/game.json`) and their pictures.
- `internal/assets`: reading the assets directory: image decoding and the artwork of the screens.
- `internal/gfx`: drawing helpers and text in the game's fonts.
- `internal/input`: keys and gamepads as actions, and the script the scenario tests play through.
- `internal/sound`: the synthesized music and sound effects.
- `internal/save`: the save data and where it is kept.

`internal/game` uses the others; none of them imports `internal/game`, and `road`, `internal/input`, `internal/sound` and `internal/save` import nothing of the game.

### 3. Keep the quality bar high

- Add or update unit tests when you add features or fix bugs. The rules of the road live in the `road` package (with the timing, each character's run of courses and the auto player in `internal/engine`) and are fully testable without a window; `internal/game/scenario_test.go` plays whole screens through a scripted input.
- Keep the game working on Linux, macOS and Windows; CI tests all three.
- Keep comments in English.

### 4. Run the checks before opening a pull request

```shell
make test    # unit tests with the race detector and coverage
make test-pixels  # reads back what the screens draw (opens a window; xvfb-run on Linux without a desktop)
make e2e     # end-to-end tests of the built binary (requires atago)
make vet
make fmt
make lint    # golangci-lint, the same configuration CI enforces
```

The end-to-end tests run the built binary through [atago](https://github.com/nao1215/atago) with the specs in `e2e/atago/`. Install atago with `go install github.com/nao1215/atago@latest`, then run `make e2e` (or `go run ./e2e/runner --filter reset` for some of the scenarios). The specs that open a window (`--capture` and `--record-demo`) run only with `RABBITRUN_E2E_DISPLAY=1`; on Linux without a desktop, use `RABBITRUN_E2E_DISPLAY=1 xvfb-run --auto-servernum make e2e`. CI runs the suite on Linux, macOS and Windows.

`make tools` installs golangci-lint, govulncheck and go-licenses at the versions CI pins. CI also runs `govulncheck` against every supported Go version and checks the licenses of every dependency with `go-licenses`:

```shell
make tools
make vuln
make licenses
```

### 5. Artwork and music

The images under `assets/` and the music data in `internal/sound/songs_data.go` are maintained by the author. If you would like a change to the artwork, please open an issue rather than sending replacement images.

### 6. Commit messages

Use [Conventional Commits](https://www.conventionalcommits.org/) prefixes (`feat:`, `fix:`, `docs:`, `test:`, `chore:`, `ci:` ...). The release notes are grouped by them.
