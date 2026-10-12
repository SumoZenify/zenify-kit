package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeSpec writes a design file under <store>/specs/<repo>/.
func writeSpec(t *testing.T, store, repo, name, body string) {
	t.Helper()
	dir := filepath.Join(store, "specs", repo)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestRunSpecStatus_EmptyStoreExitsClean(t *testing.T) {
	store := t.TempDir()
	if err := os.MkdirAll(filepath.Join(store, "specs"), 0o750); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	err := runSpecStatus(store, "", false, true, "", true,
		func(string) (string, bool) { return "", false },
		func(string) string { return "origin/main" },
		&fakeGit{}, &out, &errb)
	if err != nil {
		t.Fatalf("err = %v, want nil (fail-open)", err)
	}
}

func TestRunSpecStatus_EmptyStorePrintsMessage(t *testing.T) {
	store := t.TempDir()
	if err := os.MkdirAll(filepath.Join(store, "specs"), 0o750); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	// jsonOut=false → table mode must print the empty-state message (SC-06).
	err := runSpecStatus(store, "", false, false, "", true,
		func(string) (string, bool) { return "", false },
		func(string) string { return "origin/main" },
		&fakeGit{}, &out, &errb)
	if err != nil {
		t.Fatalf("err = %v, want nil (fail-open)", err)
	}
	if !strings.Contains(out.String(), "không có spec") { //znf:allow-lang
		t.Fatalf("empty store did not print the no-spec message (SC-06):\n%s", out.String())
	}
}

func TestRunSpecContracts_EmptyPrintsMessage(t *testing.T) {
	store := t.TempDir()
	writeSpec(t, store, "zenify-kit", "2026-09-09-b-design.md", "## Brief\n(no tags)\n")
	var out, errb bytes.Buffer
	err := runSpecContracts(store, "", "", false, &out, &errb)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(out.String(), "không có contract") { //znf:allow-lang
		t.Fatalf("empty registry did not print the no-contract message:\n%s", out.String())
	}
}

func TestRunSpecContracts_MissingTagCounted(t *testing.T) {
	store := t.TempDir()
	writeSpec(t, store, "zenify-kit", "2026-09-09-a-design.md",
		"## Brief\n_Blast-radius: zenify-kit only_\n_DB: N/A_\n")
	writeSpec(t, store, "zenify-kit", "2026-09-09-b-design.md", "## Brief\n(no tags)\n")
	var out, errb bytes.Buffer
	err := runSpecContracts(store, "", "", false, &out, &errb)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(out.String(), "zenify-kit only") {
		t.Fatalf("out missing the tagged spec:\n%s", out.String())
	}
}

// fakeSpecStore builds a two-spec store: a (with plan) and b (without).
func fakeSpecStore(t *testing.T) string {
	t.Helper()
	store := t.TempDir()
	writeSpec(t, store, "a", "2026-01-01-x-design.md", "## Brief\n(no tags)\n")
	writeSpec(t, store, "b", "2026-01-02-y-design.md", "## Brief\n(no tags)\n")
	planDir := filepath.Join(store, "plans", "a")
	if err := os.MkdirAll(planDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(planDir, "2026-01-01-x.md"), []byte("# plan\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return store
}

func TestSpecStatus_GoldenOutput(t *testing.T) {
	store := fakeSpecStore(t)
	fakeResolve := func(string) (string, bool) { return "", false }
	fakeBase := func(string) string { return "origin/main" }
	for _, c := range []struct {
		name string
		json bool
	}{{"table", false}, {"json", true}} {
		var out, errb bytes.Buffer
		if err := runSpecStatus(store, t.TempDir(), true, c.json, "", false, fakeResolve, fakeBase, &fakeGit{}, &out, &errb); err != nil {
			t.Fatal(err)
		}
		got := strings.ReplaceAll(out.String(), store, "<store>")
		golden := filepath.Join("testdata", "spec_status_"+c.name+".golden")
		if os.Getenv("UPDATE_GOLDEN") != "" {
			if err := os.WriteFile(golden, []byte(got), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		want, _ := os.ReadFile(golden)
		if got != string(want) {
			t.Fatalf("%s output drifted:\n%s", c.name, got)
		}
	}
}
