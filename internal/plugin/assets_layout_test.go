package plugin

import (
	"strings"
	"testing"
)

// FR-7.6 / FR-7.8: the doctrine names every docs/ dir; no asset passes a filename to the screenshot tool.
func TestKnowledgeDoctrine_WhereFilesGo(t *testing.T) {
	s := syncedAsset(t, "skills/_shared/knowledge-doctrine.md")
	assertContainsAll(t, "knowledge-doctrine.md", s, []string{"## Where files go", "docs/research/", "docs/notes/", "docs/scripts/", "docs/exports/", "docs/external/", "docs/specs/", "docs/plans/", "docs/reference/", "docs/handoff/", "docs/releases/", "without a `filename`"})
	if strings.Contains(s, "rule-candidates") || strings.Contains(s, "Nominate") {
		t.Error("nominate → ratify mechanism was retired (spec vòng 4)") //znf:allow-lang
	}
}

func TestWritingPlans_NoReviewerTranscript(t *testing.T) {
	s := syncedAsset(t, "skills/writing-plans/SKILL.md")
	if !strings.Contains(s, "Reviewer output") || len(s) > maxSkillBytes {
		t.Fatalf("missing rule or over budget (%d)", len(s))
	}
}
