package plugin

import (
	"strings"
	"testing"
)

// FR-7 / SC-4: brainstorming dispatches the spec reviewer and no longer routes an architect.
func TestBrainstorming_Skill(t *testing.T) {
	s := syncedAsset(t, "skills/brainstorming/SKILL.md")
	assertContainsAll(t, "brainstorming/SKILL.md", s, []string{
		"Agent(znf:spec-reviewer)", "spec review ran on:", "Think hard before responding.",
	})
	for _, banned := range []string{"architect-gate.md", "architect-memo.md", "select-route", "Agent(architect)"} {
		if strings.Contains(s, banned) {
			t.Errorf("brainstorming/SKILL.md must not contain %q", banned)
		}
	}
	if len(s) > 8192 || strings.Count(s, "\n") > 200 {
		t.Fatalf("brainstorming over cap: %d bytes, %d lines", len(s), strings.Count(s, "\n"))
	}
}
