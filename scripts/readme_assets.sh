#!/usr/bin/env bash
#
# readme_assets.sh — refresh what the README shows of the game: the picture counts in
# the character table, the screenshots and the demo GIF under doc/img.
#
#   scripts/readme_assets.sh                 # all three steps (make readme)
#   scripts/readme_assets.sh counts          # only the counts (no window, no ffmpeg)
#   scripts/readme_assets.sh shots demo      # only the images
#
# counts  rewrites the Portraits and Illustrations columns of README.md from assets/
#         (go run ./scripts/readme counts): the portraits are the expressions with a
#         picture, as the gallery lists them, and the illustrations are "R + α (S)" with
#         R the regular illustrations and S the extra ones plus the two endings.
# shots   runs `rabbitrun --capture` twice, each time with a save data of its own in a
#         temporary config directory (the player's save is never read): a fresh save for
#         the title, the character select and the play screens (the secret character is a
#         silhouette), and a save where the four regular characters have seen every
#         portrait and unlocked the regular illustrations, but cleared nothing, for the
#         gallery (still a silhouette, no extra illustrations). The screens are copied to
#         doc/img and re-encoded as 256-color PNGs. illustration.jpg is gyal's cg_selfie
#         scaled to 360 pixels wide.
# demo    records the self-playing demo of gyal on stage 1 (`rabbitrun --record-demo`) and
#         turns 8 seconds of it into doc/img/demo.gif (360 pixels wide, 12 fps). The demo
#         swings its hammer once, about four seconds in, when walls are on the screen; the
#         GIF starts 2 seconds before the cut-in of that swing, found by the pink band it
#         draws across the screen (the step fails when there is none).
#
# The screens need a display: DISPLAY (or macOS) is used when there is one, otherwise the
# game runs under xvfb-run. ffmpeg must be on PATH. The roads are built from fixed seeds,
# so a run on the same assets gives the same pictures, but for the poses the characters
# pick at random and the encoders' noise. Never put the secret character, the extra
# illustrations or the title of the last reward in the README: these saves keep them out.
set -euo pipefail

cd "$(dirname "$0")/.."
img=doc/img
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

die() { printf 'readme_assets: %s\n' "$*" >&2; exit 1; }

steps=("$@")
if [ ${#steps[@]} -eq 0 ]; then steps=(counts shots demo); fi
for s in "${steps[@]}"; do
  case "$s" in counts | shots | demo) ;; *) die "unknown step '$s' (counts, shots or demo)" ;; esac
done
want() { local s; for s in "${steps[@]}"; do [ "$s" = "$1" ] && return 0; done; return 1; }

# run_game HOME_DIR ARGS... runs the game built into $work with its save data under
# HOME_DIR, on the display there is or under xvfb-run.
run_game() {
  local home="$1"; shift
  local env=()
  if [ "$(uname -s)" = Darwin ]; then
    env=(HOME="$home") # ~/Library/Application Support/rabbitrun
  else
    env=(XDG_CONFIG_HOME="$home/.config") # ~/.config/rabbitrun
  fi
  if [ -n "${DISPLAY:-}${WAYLAND_DISPLAY:-}" ] || [ "$(uname -s)" = Darwin ]; then
    env "${env[@]}" "$work/rabbitrun" "$@"
  elif command -v xvfb-run > /dev/null; then
    env "${env[@]}" xvfb-run --auto-servernum --server-args='-screen 0 1280x1024x24' "$work/rabbitrun" "$@"
  else
    die "no display: set DISPLAY or install xvfb-run"
  fi
}

# save_file HOME_DIR is where the game reads its save data under HOME_DIR.
save_file() {
  if [ "$(uname -s)" = Darwin ]; then
    printf '%s/Library/Application Support/rabbitrun/save.json' "$1"
  else
    printf '%s/.config/rabbitrun/save.json' "$1"
  fi
}

# png256 IN OUT re-encodes the screenshot IN as a 256-color PNG at OUT.
png256() {
  ffmpeg -nostdin -loglevel error -y -i "$1" \
    -vf 'split[a][b];[a]palettegen=max_colors=256:stats_mode=full[p];[b][p]paletteuse=dither=sierra2_4a' \
    -frames:v 1 "$2"
}

if want counts; then
  echo "== counts"
  go run ./scripts/readme counts
fi

if want shots || want demo; then
  command -v ffmpeg > /dev/null || die "ffmpeg is not on PATH"
  go build -o "$work/rabbitrun" .
fi

if want shots; then
  echo "== screenshots (fresh save)"
  mkdir -p "$work/fresh" "$work/shots_fresh"
  run_game "$work/fresh" --capture "$work/shots_fresh" > /dev/null
  # README name <- capture name
  for pair in title:title select:select play:play play_cutin:play_cutin play_break:play_break play_cg:play_flash; do
    png256 "$work/shots_fresh/${pair#*:}.png" "$img/${pair%%:*}.png"
  done

  echo "== screenshots (gallery save)"
  mkdir -p "$work/gallery" "$work/shots_gallery"
  go run ./scripts/readme gallery-save "$(save_file "$work/gallery")"
  run_game "$work/gallery" --capture "$work/shots_gallery" > /dev/null
  png256 "$work/shots_gallery/gallery.png" "$img/gallery.png"

  echo "== illustration"
  ffmpeg -nostdin -loglevel error -y -i assets/characters/gyal/images/cg_selfie.jpg \
    -vf 'scale=360:-2:flags=lanczos' -q:v 3 "$img/illustration.jpg"
fi

if want demo; then
  echo "== demo"
  mkdir -p "$work/demo"
  run_game "$work/demo" --record-demo "$work/demo.mp4" --record-char gyal --record-stage 1 --record-seconds 16
  # The cut-in is a pink band across the middle of the screen: the first frame after 3
  # seconds whose band is that red (the V of YUV) is the swing.
  ffmpeg -nostdin -loglevel error -i "$work/demo.mp4" \
    -vf "crop=720:60:0:420,signalstats,metadata=print:key=lavfi.signalstats.VAVG:file=$work/band.txt" -f null -
  cut="$(awk -F'[ =:]+' '/pts_time/ { for (i = 1; i < NF; i++) if ($i == "pts_time") t = $(i + 1) }
    /VAVG/ { if (t > 3 && $NF > 145) { print t; exit } }' "$work/band.txt")"
  [ -n "$cut" ] || die "the demo swung no hammer in its first 16 seconds; the GIF would not show it"
  start="$(awk -v c="$cut" 'BEGIN { s = c - 2; if (s < 0) s = 0; printf "%.2f", s }')"
  echo "the hammer's cut-in at ${cut}s; the GIF starts at ${start}s"
  ffmpeg -nostdin -loglevel error -y -ss "$start" -t 8 -i "$work/demo.mp4" \
    -vf 'fps=12,scale=360:-1:flags=lanczos,split[a][b];[a]palettegen=max_colors=128:stats_mode=diff[p];[b][p]paletteuse=dither=bayer:bayer_scale=4:diff_mode=rectangle' \
    -loop 0 "$img/demo.gif"
fi

if want shots || want demo; then
  ls -l "$img"/*.png "$img"/*.jpg "$img"/*.gif
fi
