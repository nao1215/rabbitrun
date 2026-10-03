#!/bin/sh
# [Description]
#  Collect the license texts of every module linked into the shipped rabbitrun
#  binary into third_party_licenses/<goos>/<module path>/ for linux, darwin and
#  windows (Ebitengine links different packages per OS). GoReleaser runs this
#  as a before hook and ships the directory as THIRD_PARTY_LICENSES/ in the
#  release archives and packages.
#
#  go-licenses v2 is required (go install github.com/google/go-licenses/v2@v2.0.1).
#  GOROOT is set from `go env GOROOT` because go-licenses recognizes the
#  standard library by path prefix, and a toolchain switch would otherwise make
#  it treat every standard package as an unlicensed dependency.
set -eu

GOROOT="$(go env GOROOT)"
export GOROOT

for goos in linux darwin windows; do
  GOOS="${goos}" CGO_ENABLED=0 go-licenses save . \
    --save_path="third_party_licenses/${goos}" --force
done
