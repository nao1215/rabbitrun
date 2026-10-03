# Changelog

All notable changes to this project are documented in this file. The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Rabbit Run: a portrait (720x900) road runner built with Ebitengine. A bunny hops up a road of gummy blocks; slide her left and right, pick up macarons and stay off the walls. A run is four stages of four courses, about three minutes.
- Course themes: every course has one of twenty-one themes (a winding narrow road, wide swings from side to side, slaloms, gates, pillars that step across the road, lanes, a checkerboard, a funnel and more). Each character runs roads of her own, the same every time, and a long straight run ends with a block to dodge.
- Macarons that add up to extra lives (the first after 20, then every 150), and vaults with an extra life that only the hammer opens.
- Bonus courses: one course in each stage after the first is walled with blocks of every candy color, with a wider road, twice the sweets and a feast of macarons.
- The hammer: a squeaky toy hammer that smashes every wall on the screen, with a cut-in of the character.
- Holding up speeds the road up for the rest of the stage; the music follows the speed of the road.
- Misses on a wall from the front or the side; a retry uses a life and goes back ten rows on the same road, without the sweets already taken.
- Illustrations: each course cleared unlocks one and shows it behind the road; fifteen per character, fifteen more on the extra stages, and an ending picture for each.
- Extra stages, opened by typing `RABBITRUN` (in capitals) on the title screen: tighter, faster courses and another set of illustrations.
- Five characters whose full-body portraits react to the run (about twenty situations, several poses each). The fifth is a secret character, brought in on the title screen once the first four have cleared the game.
- Vivaldi's Four Seasons as drum and bass, one season per screen.
- Character select screen with a fanned hand of large cards, and a gallery of every portrait and illustration.
- Keyboard (arrows, WASD or the vi keys HJKL) and gamepad support.
- `--reset-save` to delete the save data and exit (the old one is kept as `save.json.bak`), `--debug` to unlock everything without touching the save data, and `--record-demo` to record a run played by itself.
- Release archives and Linux packages for Linux, macOS and Windows (amd64 and arm64), with a cosign-signed checksum file, an SBOM per archive, GitHub build provenance and the license texts of the linked Go modules under `THIRD_PARTY_LICENSES/`.
- A Homebrew cask in nao1215/tap (`brew install --cask nao1215/tap/rabbitrun`).
