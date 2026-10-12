package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ZenifyAIContactCenter/zenify-kit/internal/docsview"
)

// docs sync resolves the repo dir via discovery (repos/<repo> or flat).
func TestDocsDirResolves(t *testing.T) {
	ws := t.TempDir()
	repo := filepath.Join(ws, "docs")
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	got := resolveWorkspaceRepoDir(ws, defaultDocsRepo, "", os.ReadDir)
	if filepath.Clean(got) != filepath.Clean(repo) {
		t.Fatalf("resolve docs dir: got %q want %q", got, repo)
	}
}

// --dir override must win: resolveDocsStore must not be consulted, and the store
// actually used is exactly --dir (not the path resolveDocsStore would return),
// shown via the link farm created under workspace/docs → --dir/specs.
func TestDocsSyncDirFlagWins(t *testing.T) {
	ws := t.TempDir()
	store := t.TempDir() // NOT a git repo, NOT under ws — entirely different from where resolveDocsStore would point
	if err := os.MkdirAll(filepath.Join(store, "specs"), 0o755); err != nil {
		t.Fatal(err)
	}

	cmd := newDocsCmd()
	var errBuf bytes.Buffer
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{"sync", "--workspace", ws, "--dir", store})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// viewDir = ws/docs, must differ from store so EnsureView runs and links to store/specs.
	viewDir := filepath.Join(ws, defaultDocsRepo)
	if _, err := os.Stat(filepath.Join(viewDir, "specs")); err != nil {
		t.Fatalf("view farm not created at %s: %v (--dir may NOT have won)", viewDir, err)
	}
	same, err := (docsview.OSFS{}).SameTarget(filepath.Join(viewDir, "specs"), filepath.Join(store, "specs"))
	if err != nil || !same {
		t.Fatalf("view/specs does not point to --dir/specs: same=%v err=%v", same, err)
	}
}

// EnsureView must NOT run when viewDir == dir (not migrated: docs store still
// lives right in the workspace) — no "docs view:" note is printed.
func TestDocsSyncEnsureViewSkippedWhenViewEqualsDir(t *testing.T) {
	ws := t.TempDir()
	dir := filepath.Join(ws, defaultDocsRepo) // == viewDir
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	cmd := newDocsCmd()
	var errBuf bytes.Buffer
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{"sync", "--workspace", ws, "--dir", dir})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if strings.Contains(errBuf.String(), "docs view:") {
		t.Fatalf("EnsureView must be skipped when viewDir==dir, but got note: %s", errBuf.String())
	}
}

func TestStoreDirsPinned(t *testing.T) {
	want := []string{"specs", "plans", "reference", "handoff", "releases"}
	if !reflect.DeepEqual(storeDirs, want) {
		t.Fatalf("storeDirs = %v, want %v", storeDirs, want)
	}
	if len(docsDirs) != 10 {
		t.Fatalf("docsDirs = %v", docsDirs)
	}
}

// SC-1 (migrated): five local dirs are real dirs, five store dirs are links; second run adds no note.
func TestDocsSyncCreatesLocalDirs(t *testing.T) {
	ws, store := t.TempDir(), t.TempDir()
	os.MkdirAll(filepath.Join(store, "specs"), 0o755)
	run := func() string {
		cmd := newDocsCmd()
		var e bytes.Buffer
		cmd.SetErr(&e)
		cmd.SetArgs([]string{"sync", "--workspace", ws, "--dir", store})
		cmd.Execute()
		return e.String()
	}
	run()
	for _, d := range localDirs {
		fi, err := os.Lstat(filepath.Join(ws, "docs", d))
		if err != nil || !fi.IsDir() || fi.Mode()&os.ModeSymlink != 0 {
			t.Fatalf("%s: %v %v", d, fi, err)
		}
	}
	if fi, _ := os.Lstat(filepath.Join(ws, "docs", "specs")); fi.Mode()&os.ModeSymlink == 0 {
		t.Fatal("specs must be a link")
	}
	if out := run(); strings.Contains(out, "docs local:") {
		t.Fatalf("second run noted: %s", out)
	}
}

// SC-1 (not migrated): viewDir == store, five dirs created inside the store.
func TestDocsSyncLocalDirsWhenViewEqualsDir(t *testing.T) {
	ws := t.TempDir()
	dir := filepath.Join(ws, "docs")
	os.MkdirAll(dir, 0o755)
	cmd := newDocsCmd()
	cmd.SetArgs([]string{"sync", "--workspace", ws, "--dir", dir})
	cmd.Execute()
	for _, d := range localDirs {
		if fi, err := os.Stat(filepath.Join(dir, d)); err != nil || !fi.IsDir() {
			t.Fatalf("%s missing", d)
		}
	}
}
