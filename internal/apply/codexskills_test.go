package apply

import (
	"os"
	"path/filepath"
	"testing"
)

func codexSkillsFixture(t *testing.T) (home, link, target string) {
	t.Helper()
	home = t.TempDir()
	target = filepath.Join(home, ".claude", "skills", "znf")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	return home, filepath.Join(home, ".agents", "skills", "znf"), target
}

func TestEnsureCodexSkillsLink_CreatedThenUnchanged(t *testing.T) {
	home, link, target := codexSkillsFixture(t)
	st, err := EnsureCodexSkillsLink(home, false)
	if err != nil || st != LinkCreated {
		t.Fatalf("first: st=%v err=%v", st, err)
	}
	if got, _ := os.Readlink(link); got != target {
		t.Fatalf("readlink=%q want %q", got, target)
	}
	if st, err = EnsureCodexSkillsLink(home, false); err != nil || st != LinkUnchanged {
		t.Fatalf("second: st=%v err=%v", st, err)
	}
}

func TestEnsureCodexSkillsLink_RewritesWrongTarget(t *testing.T) {
	home, link, target := codexSkillsFixture(t)
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/elsewhere", link); err != nil {
		t.Fatal(err)
	}
	st, err := EnsureCodexSkillsLink(home, false)
	if err != nil || st != LinkRewritten {
		t.Fatalf("st=%v err=%v", st, err)
	}
	if got, _ := os.Readlink(link); got != target {
		t.Fatalf("readlink=%q want %q", got, target)
	}
}

func TestEnsureCodexSkillsLink_RealDirUntouched(t *testing.T) {
	home, link, _ := codexSkillsFixture(t)
	if err := os.MkdirAll(link, 0o755); err != nil {
		t.Fatal(err)
	}
	st, err := EnsureCodexSkillsLink(home, false)
	if err != nil || st != LinkRealDir {
		t.Fatalf("st=%v err=%v", st, err)
	}
	if fi, _ := os.Lstat(link); fi == nil || !fi.IsDir() || fi.Mode()&os.ModeSymlink != 0 {
		t.Fatal("real dir must stay a real dir")
	}
}

func TestEnsureCodexSkillsLink_DryRunCreatesNothing(t *testing.T) {
	home, _, _ := codexSkillsFixture(t)
	st, err := EnsureCodexSkillsLink(home, true)
	if err != nil || st != LinkCreated {
		t.Fatalf("st=%v err=%v", st, err)
	}
	if _, err := os.Lstat(filepath.Join(home, ".agents")); !os.IsNotExist(err) {
		t.Fatalf("dry-run created .agents: %v", err)
	}
}

func TestRemoveCodexSkillsLink(t *testing.T) {
	home, link, _ := codexSkillsFixture(t)
	if _, err := EnsureCodexSkillsLink(home, false); err != nil {
		t.Fatal(err)
	}
	if ok, err := RemoveCodexSkillsLink(home, true); err != nil || !ok {
		t.Fatalf("dry: ok=%v err=%v", ok, err)
	}
	if _, err := os.Lstat(link); err != nil {
		t.Fatal("dry-run must not remove")
	}
	if ok, err := RemoveCodexSkillsLink(home, false); err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if _, err := os.Lstat(link); !os.IsNotExist(err) {
		t.Fatal("link should be gone")
	}
	// wrong target is kept
	if err := os.Symlink("/elsewhere", link); err != nil {
		t.Fatal(err)
	}
	if ok, err := RemoveCodexSkillsLink(home, false); err != nil || ok {
		t.Fatalf("wrong target: ok=%v err=%v", ok, err)
	}
	if _, err := os.Lstat(link); err != nil {
		t.Fatal("foreign symlink must stay")
	}
}
