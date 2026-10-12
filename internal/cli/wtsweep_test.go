package cli

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"testing"
)

func TestWtSweep_DryRunIntegration(t *testing.T) {
	requireGit(t)
	t.Setenv("WT_SESSION", "")
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	root := initGitRepo(t)
	t.Setenv("WT_REPO_ROOT", root)
	if _, err := runWt(t, root, "new", "sweepable", "--type", "feat"); err != nil {
		t.Fatalf("precondition wt new: %v", err)
	}
	// initGitRepo's worktree.json sets baseRef=main (local), and a freshly-created
	// branch sits at main's tip → BranchMerged true → clean → dry-run must list it
	// as would-remove, and remove NOTHING.
	out, err := runWt(t, root, "sweep", "--dry-run")
	if err != nil {
		t.Fatalf("sweep --dry-run: %v", err)
	}
	if !strings.Contains(out, "sweepable") || !strings.Contains(out, "would") {
		t.Fatalf("dry-run should list sweepable as would-remove: %q", out)
	}
	// worktree still present after a dry-run
	ls, _ := runWt(t, root, "ls")
	if !strings.Contains(ls, "sweepable") {
		t.Fatalf("dry-run must not remove the worktree: %q", ls)
	}
}

func TestWtSweep_CallsDocsSweep(t *testing.T) { // SC-11
	requireGit(t)
	t.Setenv("WT_SESSION", "")
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	root := initGitRepo(t)
	t.Setenv("WT_REPO_ROOT", root)
	writeF(t, filepath.Join(root, ".zenify", "manifest.json"), "{}") // root is a workspace
	var got []string
	old := sweepDocs
	sweepDocs = func(ws string, apply, noFetch bool, _ io.Writer) error {
		got = append(got, fmt.Sprintf("%s apply=%v noFetch=%v", ws, apply, noFetch))
		return nil
	}
	t.Cleanup(func() { sweepDocs = old })
	if _, err := runWt(t, root, "sweep", "--dry-run"); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || !strings.HasSuffix(got[0], "apply=false noFetch=true") || !strings.HasPrefix(got[0], root) {
		t.Fatalf("got %v", got)
	}
}

func TestWtSweep_NoWorkspaceSkipsDocsSweep(t *testing.T) {
	requireGit(t)
	t.Setenv("WT_SESSION", "")
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	root := initGitRepo(t)
	t.Setenv("WT_REPO_ROOT", root)
	called := false
	old := sweepDocs
	sweepDocs = func(string, bool, bool, io.Writer) error { called = true; return nil }
	t.Cleanup(func() { sweepDocs = old })
	runWt(t, root, "sweep", "--dry-run")
	if called {
		t.Fatal("docs sweep must not run outside a workspace")
	}
}
