#!/usr/bin/env bash
#
# smoke_artifacts.sh — check the GoReleaser output in <dist> before it ships.
#
# For every release target it checks that the archive exists, contains the
# binary and the license files, and that checksums.txt covers it. The binary
# built for the runner's own OS/arch is also executed with --version, which needs
# no display, to prove it starts.
set -euo pipefail

dist="${1:-dist}"
failures=0
fail() { printf 'FAIL: %s\n' "$*"; failures=$((failures + 1)); }

host_os="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$(uname -m)" in
  x86_64|amd64) host_arch="amd64" ;;
  arm64|aarch64) host_arch="arm64" ;;
  *) host_arch="unknown" ;;
esac

[ -f "${dist}/checksums.txt" ] || { echo "no ${dist}/checksums.txt" >&2; exit 1; }

for os in linux darwin windows; do
  for arch in amd64 arm64; do
    if [ "$os" = "windows" ]; then ext="zip"; bin="rabbitrun.exe"; else ext="tar.gz"; bin="rabbitrun"; fi
    archive="$(find "$dist" -maxdepth 1 -name "rabbitrun_*_${os}_${arch}.${ext}" | head -n1)"
    if [ -z "$archive" ]; then fail "missing archive for ${os}/${arch}"; continue; fi
    name="$(basename "$archive")"
    grep -q " ${name}\$" "${dist}/checksums.txt" || fail "${name} is not in checksums.txt"

    work="$(mktemp -d)"
    if [ "$ext" = "zip" ]; then unzip -q "$archive" -d "$work"; else tar -xzf "$archive" -C "$work"; fi
    for f in "$bin" LICENSE NOTICE.md README.md; do
      [ -f "${work}/${f}" ] || fail "${name} does not contain ${f}"
    done
    [ -f "${work}/THIRD_PARTY_LICENSES/github.com/hajimehoshi/ebiten/v2/LICENSE" ] \
      || fail "${name} does not ship the dependency licenses in THIRD_PARTY_LICENSES/"

    if [ "$os" = "$host_os" ] && [ "$arch" = "$host_arch" ]; then
      out="$("${work}/${bin}" --version)" || fail "${name}: ${bin} --version exited non-zero"
      case "$out" in
        rabbitrun\ v*) printf 'ok:   %s runs (%s)\n' "$name" "$out" ;;
        *) fail "${name}: unexpected --version output: ${out}" ;;
      esac
    else
      printf 'ok:   %s\n' "$name"
    fi
    rm -rf "$work"
  done
done

for pkg in deb rpm apk; do
  if [ "$host_os" = "linux" ]; then
    n="$(find "$dist" -maxdepth 1 -name "*.${pkg}" | wc -l)"
    [ "$n" -ge 2 ] || fail "expected .${pkg} packages for amd64 and arm64, found ${n}"
  fi
done

if [ "$failures" -ne 0 ]; then
  printf '%d check(s) failed\n' "$failures" >&2
  exit 1
fi
printf 'all artifact checks passed\n'
