package plugin

import (
	"io/fs"
	"strings"
	"testing"
)

// FR-2.3: Codex does not substitute $ARGUMENTS, so no skill text may carry it.
func TestNoArgumentsPlaceholder(t *testing.T) {
	err := fs.WalkDir(ZnfFS(), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, rerr := fs.ReadFile(ZnfFS(), p)
		if rerr != nil {
			return rerr
		}
		if strings.Contains(string(b), "$ARGUMENTS") {
			t.Errorf("%s still contains $ARGUMENTS", p)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// FR-2.4: harness-tools.md carries a Codex column and stays within its budget.
func TestHarnessToolsBudget(t *testing.T) {
	b, err := fs.ReadFile(ZnfFS(), "skills/_shared/harness-tools.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(b) > 4096 {
		t.Errorf("harness-tools.md is %d bytes, budget 4096", len(b))
	}
	if !strings.Contains(string(b), "| Codex |") {
		t.Error("harness-tools.md table header lacks a `| Codex |` column")
	}
}
