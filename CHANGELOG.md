# Changelog

All notable changes to this project are documented in this file. The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed
- A retry now puts back exactly the road that was on the screen ten rows before the miss. On the first course it came back one row short with other macarons, so some she had already taken were there again and others were gone, and on the courses after a bonus feast or a vault in the same stage the lines of macarons and the hammer had moved.

## [0.1.0] - 2026-10-04

### Added
- Rabbit Run: a portrait (720x900) road runner built with Ebitengine. A bunny hops up a road of gummy blocks; slide her left and right, pick up macarons and stay off the walls. A run is four stages of four courses, about three minutes.
- Course themes: every course has one of thirty-one themes (a winding narrow road, wide swings from side to side, slaloms, gates, pillars that step across the road, lanes, a checkerboard, a funnel and more). Each character runs roads of her own, the same every time, and a long straight run ends with a block to dodge.
- Macarons that add up to extra lives (every 100), some laid in lines along the way, and vaults with an extra life that only the hammer opens.
- Bonus courses: one course in each stage after the first is walled with blocks of every candy color, with a wider road, twice the sweets and a feast of macarons.
- The hammer: a squeaky toy hammer that smashes every wall on the screen, with a cut-in of the character.
- On the first stage of the regular side, a road that would let her stand still for more than half a screen puts one block in her way, with a step around it; the other courses are the roads they were.
- Holding up speeds the road up until a miss; the music follows the speed of the road.
- Misses on a wall from the front or the side; a retry uses a life and goes back ten rows on the same road, without the sweets already taken.
- Illustrations: each course cleared unlocks one and shows it behind the road; fifteen per character, fifteen more on the extra stages, and an ending picture for each.
- Extra stages, opened by typing `RABBITRUN` (in capitals) on the title screen: tighter, faster courses and another set of illustrations.
- Five characters whose full-body portraits react to the run (about twenty situations, several poses each). The fifth is a secret character, brought in on the title screen once the first four have cleared the game.
- Vivaldi's Four Seasons as drum and bass, one season per screen.
- Character select screen with a fanned hand of large cards, and a gallery of every portrait and illustration.
- Keyboard (arrows, WASD or the vi keys HJKL) and gamepad support.
- `--reset-save` to delete the save data and exit (the old one is kept as `save.json.bak`), `--debug` to unlock everything without touching the save data, and `--record-demo` to record a run played by itself (`--record-extra` records it on the extra stages).
- Command-line mistakes (an unknown option, a stray argument, two jobs such as `--bgm-wav` and `--reset-save` at once, a `--record-*` option without `--record-demo`, a stage outside 1 to 4, an unknown character, an empty directory) exit with status 2 before the game opens, with the error first and a pointer to `--help`. `--record-demo` checks for ffmpeg before opening a window, `--bgm-wav` stops with status 1 at the first file it cannot write, `--capture` and `--record-demo` never change the save data, and `--reset-save` says so when there is no save to reset.
- End-to-end tests of the built binary with [atago](https://github.com/nao1215/atago) (`make e2e`), run by CI on Linux, macOS and Windows.
- Release archives and Linux packages for Linux, macOS and Windows (amd64 and arm64), with a cosign-signed checksum file, an SBOM per archive, GitHub build provenance and the license texts of the linked Go modules under `THIRD_PARTY_LICENSES/`.
- A Homebrew cask in nao1215/tap (`brew install --cask nao1215/tap/rabbitrun`).
- Eight course themes built around one idea each that can be read well ahead, with their macarons, hammers and extra lives laid as part of the design (one of them a fork whose hard lane pays off), spread over every character's runs.
- Two more course themes: a side lane beside a safe one that gets harder and pays more the deeper she goes, with ways back out, and a lesson that shows three moves one at a time before stringing them into a phrase and mirroring it.
- Difficulty and fun scores for every course, measured on the road itself (narrow rows, sideways slides, rests, blocks to aim past, speed, help; sweets, items, rewards off the easy line, feasts, how much the road repeats), with tests that keep the runs harder from left to right on the character select screen, the extra side harder than the regular side course by course, every run harder stage by stage, every stage fun enough and no theme twice in a row.

### Changed
- The characters' courses were retuned to those scores: the cool girl's runs are now the easiest and the bunny girl's the hardest, and the checkerboards stay where they were.
- Every character's road runs at the same speed (only the extra stages are faster), so the controls feel the same whoever runs; the order of difficulty comes from the courses alone.
- The comb leaves out one tooth where its teeth change walls, and scattered blocks keep clear of a block of the two rows before, so neither asks for a dash of two cells on a fast road.

### Fixed
- `--debug` no longer writes the save data: a run with it saved the secret character as announced, so when the four regular characters later cleared for real the title never brought her in. Saves already marked that way by `--debug` show her arrival once she is earned.
- The bunny no longer drops back from a wall she slides into from the side: the road kept scrolling for the frame of the miss and could jump back a row.
- The checkerboard courses (the cool girl's, the gyaru's and the bunny girl's) leave out a block now and then, so there is a spot to stand still in instead of a step aside for every row of blocks, and each lays a hammer as its checker rows begin and an extra life halfway through.
- The cool girl, the one to start with, finds one more hammer on her road early in the first stage.
- Closing the window in the middle of a run (playing, paused or after a miss) records the stage reached and the time played, as quitting through the pause menu does; they were lost.
- A save that fails to write (a full disk, a folder without write permission) is tried again on the next frame and when the game exits, and the failure is logged once; the change was dropped, so the save stayed behind until something else changed.
- `--record-demo` exits with status 1 when ffmpeg fails or stops reading the frames, and `--capture` stops with status 1 at the first screenshot it cannot save; both were only logged, and the game exited 0 without the video or with screens missing.
- A second retry right after the first no longer brings back the macarons, hammers and extra lives already taken: it goes back further than the screen, and the rows built again as the road came on were not cleared of them.
