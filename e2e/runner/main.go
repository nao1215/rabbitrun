// Command runner bootstraps rabbitrun's end-to-end suite and hands the specs to
// atago.
//
// It builds rabbitrun from this checkout into a throwaway directory, with the
// version set to v0.0.0-e2e through the same linker flag a release uses, puts that
// directory first on PATH so the specs exercise that exact binary, and runs the
// atago specs under e2e/atago.
//
// The test definitions are the atago YAML; this program is only the environment
// bootstrap. It is Go rather than a shell script because the suite runs on Windows
// too, and a bash bootstrap would make the Windows leg depend on Git Bash being
// installed, which tests the runner image rather than rabbitrun.
//
// Environment the specs read:
//
//	RABBITRUN_E2E_BIN_DIR  set by the runner: the directory holding the binary under
//	                       test (a spec puts only this on PATH to hide ffmpeg)
//	RABBITRUN_E2E_DISPLAY  set by the caller when a display is available (CI sets
//	                       it under xvfb-run on Linux); the specs that open a window
//	                       (--capture, --record-demo) run only when it is set
//
// Usage:
//
//	go run ./e2e/runner                  # every spec under e2e/atago
//	go run ./e2e/runner --filter reset   # extra flags are passed through to atago
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// e2eVersion is the version the binary under test is built with; the specs pin it.
const e2eVersion = "v0.0.0-e2e"

func main() {
	log.SetFlags(0)
	log.SetPrefix("e2e: ")
	if err := run(context.Background(), os.Args[1:]); err != nil {
		log.Print(err)
		var exit *exitError
		if errors.As(err, &exit) {
			os.Exit(exit.code)
		}
		os.Exit(1)
	}
}

// exitError carries an exit status out of run, so a failing atago run exits with
// atago's own status instead of a generic 1. CI reads that status to tell "the
// suite failed" from "the bootstrap broke".
type exitError struct {
	code int
	err  error
}

func (e *exitError) Error() string { return e.err.Error() }
func (e *exitError) Unwrap() error { return e.err }

func run(ctx context.Context, args []string) error {
	repoRoot, err := findRepoRoot()
	if err != nil {
		return err
	}

	if _, err := exec.LookPath("atago"); err != nil {
		return &exitError{code: 127, err: fmt.Errorf(
			"atago is not installed. Install it from https://github.com/nao1215/atago\n"+
				"e2e: e.g. 'go install github.com/nao1215/atago@latest' (CI uses nao1215/setup-atago): %w", err)}
	}

	tmp, err := os.MkdirTemp("", "rabbitrun-e2e-")
	if err != nil {
		return fmt.Errorf("can not create the e2e temp tree: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(tmp); err != nil {
			log.Printf("can not remove the e2e temp tree: %v", err)
		}
	}()

	binDir := filepath.Join(tmp, "bin")
	if err := os.MkdirAll(binDir, 0o750); err != nil {
		return fmt.Errorf("can not create the e2e bin directory: %w", err)
	}

	binary := filepath.Join(binDir, "rabbitrun")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	if err := buildRabbitrun(ctx, repoRoot, binary); err != nil {
		return err
	}

	// Put the e2e-built rabbitrun first on PATH so the specs resolve to that exact
	// binary rather than one the developer happens to have installed.
	if err := os.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH")); err != nil {
		return fmt.Errorf("can not extend PATH: %w", err)
	}
	if err := os.Setenv("RABBITRUN_E2E_BIN_DIR", binDir); err != nil {
		return fmt.Errorf("can not export RABBITRUN_E2E_BIN_DIR: %w", err)
	}

	version, err := exec.CommandContext(ctx, binary, "--version").Output() //nolint:gosec // the path is one this program just built
	if err != nil {
		return fmt.Errorf("can not run the freshly built rabbitrun: %w", err)
	}
	log.Print(strings.TrimSpace(firstLine(string(version))))
	if os.Getenv("RABBITRUN_E2E_DISPLAY") == "" {
		log.Print("RABBITRUN_E2E_DISPLAY is not set; the specs that open a window (--capture, --record-demo) are skipped")
	}

	// Extra args (e.g. --filter X) come before the path so atago's flag parser
	// sees them as flags rather than targets.
	atagoArgs := append([]string{"run"}, args...)
	if !hasTarget(args) {
		atagoArgs = append(atagoArgs, filepath.Join(repoRoot, "e2e", "atago"))
	}

	atago := exec.CommandContext(ctx, "atago", atagoArgs...) //nolint:gosec // a fixed command with author-supplied spec targets
	atago.Dir = repoRoot
	atago.Stdout = os.Stdout
	atago.Stderr = os.Stderr
	atago.Stdin = os.Stdin
	if err := atago.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return &exitError{code: exit.ExitCode(), err: fmt.Errorf("atago run failed: %w", err)}
		}
		return fmt.Errorf("can not run atago: %w", err)
	}
	return nil
}

// buildRabbitrun compiles the binary under test, with the version set the way a
// release sets it (Makefile, .goreleaser.yaml).
func buildRabbitrun(ctx context.Context, repoRoot, output string) error {
	log.Print("building rabbitrun...")
	cmd := exec.CommandContext(ctx, "go", "build", "-ldflags", "-X main.version="+e2eVersion, "-o", output, ".") //nolint:gosec // fixed arguments and a path this program chose
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("can not build rabbitrun: %w", err)
	}
	return nil
}

// findRepoRoot walks up from the working directory to the checkout root. go run
// compiles into a temp directory, so os.Executable is unreliable here; the
// module root is found by looking for go.mod above the current directory.
func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("can not determine the working directory: %w", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("can not find the repository root (no go.mod above the working directory)")
		}
		dir = parent
	}
}

// valueFlags are the `atago run` options that take their value as the NEXT
// argument. Without this list the runner cannot tell `--filter reset` (a flag
// and its value) from `--filter` followed by a spec path, and would then skip
// adding the default target, silently running nothing.
var valueFlags = map[string]bool{
	"artifacts-dir": true,
	"filter":        true,
	"parallel":      true,
	"profile":       true,
	"repeat":        true,
	"report":        true,
	"retry-failed":  true,
	"skip-tag":      true,
	"tag":           true,
}

// hasTarget reports whether the caller named spec files or directories of their
// own, as opposed to passing only atago flags.
func hasTarget(args []string) bool {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") {
			return true
		}
		name := strings.TrimLeft(arg, "-")
		if strings.Contains(name, "=") {
			continue
		}
		if valueFlags[name] {
			i++
		}
	}
	return false
}

// firstLine returns everything before the first newline, so a multi-line version
// banner prints as one line.
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
