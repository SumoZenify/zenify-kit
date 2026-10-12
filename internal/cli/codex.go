package cli

import (
	"os/exec"
	"path/filepath"
)

// lookPath is a var so tests can fake the presence of the codex binary.
var lookPath = exec.LookPath

func codexOnPath() bool { _, err := lookPath("codex"); return err == nil }

// codexHomeDir is $CODEX_HOME when set, else <home>/.codex.
func codexHomeDir(home string, getenv func(string) string) string {
	if v := getenv("CODEX_HOME"); v != "" {
		return v
	}
	return filepath.Join(home, ".codex")
}
