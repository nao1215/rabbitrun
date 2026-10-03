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

### 3. Keep the quality bar high

- Add or update unit tests when you add features or fix bugs. The game rules live in `engine.go` and are fully testable without a window.
- Keep the game working on Linux, macOS and Windows; CI tests all three.
- Keep comments in English.

### 4. Run the checks before opening a pull request

```shell
make test    # unit tests with the race detector and coverage
make vet
make fmt
make lint    # golangci-lint, the same configuration CI enforces
```

`make tools` installs golangci-lint, govulncheck and go-licenses at the versions CI pins. CI also runs `govulncheck` against every supported Go version and checks the licenses of every dependency with `go-licenses`:

```shell
make tools
make vuln
make licenses
```

### 5. Artwork and music

The images under `assets/` and the music data in `songs_data.go` are maintained by the author. If you would like a change to the artwork, please open an issue rather than sending replacement images.

### 6. Commit messages

Use [Conventional Commits](https://www.conventionalcommits.org/) prefixes (`feat:`, `fix:`, `docs:`, `test:`, `chore:`, `ci:` ...). The release notes are grouped by them.
