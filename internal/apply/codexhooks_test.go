package apply

import (
	"strings"
	"testing"
)

func TestCodexEntriesDropsReadGuardAndMapsMatchers(t *testing.T) {
	got := codexEntries(znfHookSpecs())
	if len(got) != 7 {
		t.Fatalf("want 7 entries, got %d", len(got))
	}
	for _, s := range got {
		if s.ID == "read-guard" {
			t.Fatal("read-guard must be dropped")
		}
		if strings.Contains(s.Matcher, "Read") || strings.Contains(s.Matcher, "Task") || strings.Contains(s.Matcher, "Agent") {
			t.Fatalf("matcher %q still names a Claude tool", s.Matcher)
		}
	}
	byID := map[string]hookSpec{}
	for _, s := range got {
		byID[s.ID] = s
	}
	if byID["observe-count"].Matcher != codexSubagentTool {
		t.Fatalf("observe-count matcher %q", byID["observe-count"].Matcher)
	}
	if byID["observe-meter"].Matcher != codexSubagentTool+"|Bash" {
		t.Fatalf("observe-meter matcher %q", byID["observe-meter"].Matcher)
	}
	if byID["docs-sync"].command() != "zenify hooks-run docs-sync" {
		t.Fatal("command must be unchanged")
	}
}

func TestHookSpecRuntimes(t *testing.T) {
	if got := (hookSpec{ID: "read-guard"}).Runtimes(); got != "Claude" {
		t.Fatalf("read-guard runtimes = %q", got)
	}
	if got := (hookSpec{ID: "docs-sync"}).Runtimes(); got != "Claude, Codex" {
		t.Fatalf("docs-sync runtimes = %q", got)
	}
	if n := len(CodexHookSpecs()); n != 7 {
		t.Fatalf("CodexHookSpecs len = %d", n)
	}
}
