#!/usr/bin/env bash
#
# run.sh is a thin wrapper for running the end-to-end suite by path. The bootstrap
# itself is e2e/runner, a Go program: the suite runs on Windows too, and a bash
# bootstrap would make the Windows leg depend on Git for Windows being installed.
#
# Usage: e2e/run.sh [atago args...]        (e.g. e2e/run.sh --filter reset)
# The specs that open a window run only with RABBITRUN_E2E_DISPLAY set, e.g.
#   RABBITRUN_E2E_DISPLAY=1 xvfb-run --auto-servernum e2e/run.sh
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "$SCRIPT_DIR/.." && pwd)"

cd "$REPO_ROOT"
exec go run ./e2e/runner "$@"
