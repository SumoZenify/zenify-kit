package plugin

import (
	"io/fs"
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

// FR-7.8: no asset passes a filename to the screenshot tool.
func TestAssets_ScreenshotNeverPassesFilename(t *testing.T) {
	err := fs.WalkDir(assets, embedRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(p, ".md") {
			return nil
		}
		b, err := fs.ReadFile(assets, p)
		if err != nil {
			return err
		}
		for i, line := range strings.Split(string(b), "\n") {
			if strings.Contains(line, "browser_take_screenshot") && (strings.Contains(line, "filename:") || strings.Contains(line, `"filename"`)) {
				t.Errorf("%s:%d passes filename to the screenshot tool: %s", p, i+1, line)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestWritingPlans_NoReviewerTranscript(t *testing.T) {
	s := syncedAsset(t, "skills/writing-plans/SKILL.md")
	if !strings.Contains(s, "Reviewer output") || len(s) > maxSkillBytes {
		t.Fatalf("missing rule or over budget (%d)", len(s))
	}
}
