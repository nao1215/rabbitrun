# Testing

`make test` runs the unit tests with the race detector, `make test-long` the tests that play and search every whole run, `make test-pixels` the tests that read back what the screens draw, and `make e2e` the atago specs against the built binary. CONTRIBUTING.md lists them all.

## Unit, scenario and pixel tests

Every package has its tests beside it. `internal/engine` and `road/` hold the rules: the difficulty rises stage by stage and across the character select (`difficulty_rules_test.go`), the designed themes keep their shape (`road/designed_test.go`), and the careful player clears every run at full speed (`TestCarefulPlayerClearsAtFullSpeed`; the whole-run searches run in the `LongTests` workflow, also nightly). `internal/game` plays the scenes through their flows, and its pixel tests (`-tags pixels`) read back what the screens draw.

## Differential tests

`TestSlideOverIsMove` (`internal/engine/passability_test.go`) checks the careful player's slide against `road.Game.Move` on 20,000 random rows of walls, places and slides: the same place and the same miss. `TestFramesAheadAreTheFramesToCome` checks the careful player's reading of the road against the frames that then come.

## Property and metamorphic tests

`internal/save/property_test.go` draws save data with [rapid](https://pkg.go.dev/pgregory.net/rapid): data written the way the game writes it (Mark, then Flush) comes back whole in the next launch, with the unsaved extra mode off, and a read-only run (`--capture`, `--record-demo`, `--debug`) leaves the file as it was whatever it does to the data.

```sh
go test ./internal/save/ -run '^Test(Property|Metamorphic)_' -rapid.checks=20000
```

The `Property` workflow draws 20,000 inputs per test every night.

## Fuzzing

`FuzzSaveDecode` (`internal/save`), `FuzzEngineOps` (`internal/engine`) and `FuzzRetriesKeepWhatWasTaken` (`road/`) are searched every night by the `Fuzz` workflow.

## Mutation testing

[gremlins](https://github.com/go-gremlins/gremlins) v0.6.0 changes one comparison or arithmetic operator at a time and runs the package's tests; a mutant no test notices is a line the tests execute without checking. The `Mutation` workflow mutates the game's rules (`road/`, `internal/engine/`, `internal/save/`): the lines a pull request changes, and all of them once a week. The screens, sound and pictures are left out: one run of `internal/game`'s tests takes about 40 seconds.

```sh
gremlins unleash --workers 4 --timeout-coefficient 30 \
  --exclude-files '^(internal/(assets|character|game|gfx|input|sound)|e2e|scripts|tools)/' --exclude-files '^[^/]+\.go$' .
```

Measured on 2026-11-19, the first full run, with an 8 GiB memory cap per process:

| Package | Killed | Lived | Not covered | Timed out | Efficacy |
|---|---|---|---|---|---|
| `internal/save` | 17 | 1 | 8 | 0 | 94.44% |
| `internal/engine` | 173 | 91 | 12 | 8 | 65.53% |
| `road` | 664 | 990 | 169 | 18 | 40.15% |
| All | 854 | 1082 | 189 | 26 | 44.11% |

Half of the survivors (503) are in `road/themes.go`, the numbers that lay out each designed theme (widths, offsets, how often a pillar or a sweet appears); `road.go` (253) and `idle.go` (187) follow. Many of those numbers can move a little without any rule breaking, so not every survivor is a missing test, but a test that pins a theme's layout for a fixed seed would kill most of them. The whole-run searches (`RABBITRUN_LONG_TESTS=1`) are not part of the mutation run. A pull request should not lower the efficacy below this table.
