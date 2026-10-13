package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ZenifyAIContactCenter/zenify-kit/internal/routelog"
)

func TestRunRouteLogRecord_WritesThenFailOpen(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "rl")
	dirFn := func() (string, error) { return dir, nil }
	rec := `{"ts":"2026-09-21T01:00:00Z","repo":"kit","branch":"b","site":"architect","features":{"TIER":"architectural","SHARED":"1"},"gates":["SHARED"],"model":"opus","strong":"opus","changed_decision":"yes"}`
	var errb bytes.Buffer
	if err := runRouteLogRecord(strings.NewReader(rec), &errb, dirFn, os.Getenv); err != nil {
		t.Fatalf("valid record: %v", err)
	}
	if err := runRouteLogRecord(strings.NewReader(""), &errb, dirFn, os.Getenv); err != nil {
		t.Fatalf("empty must be nil: %v", err)
	}
	if err := runRouteLogRecord(strings.NewReader("{bad"), &errb, dirFn, os.Getenv); err != nil {
		t.Fatalf("malformed must be nil: %v", err)
	}
	if err := runRouteLogRecord(strings.NewReader(rec), &errb, func() (string, error) { return "", os.ErrNotExist }, os.Getenv); err != nil {
		t.Fatalf("dir error must be nil: %v", err)
	}
	recs, _ := routelog.LoadRecords(dir)
	if len(recs) != 1 {
		t.Fatalf("want exactly 1 record, got %d", len(recs))
	}
}

func TestRunRouteLogRecordPlan(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "rl")
	plan := filepath.Join(t.TempDir(), "p.md")
	body := "### Task 1: A\n\n**Files:**\n- Create: `x.go`\n- Test: `x_test.go`\n\n### Task 2: B\n\n**Files:**\n- Modify: `x.go:1-2`\n"
	if err := os.WriteFile(plan, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	var errb bytes.Buffer
	git := func(...string) ([]byte, error) { return []byte("feat/x\n"), nil }
	if err := runRouteLogRecordPlan(plan, 7, &errb, func() (string, error) { return dir, nil }, git); err != nil {
		t.Fatal(err)
	}
	recs, _ := routelog.LoadRecords(dir)
	if len(recs) != 1 || recs[0].Site != "plan-metrics" || recs[0].Tasks != 2 || recs[0].FilesPred != 2 || recs[0].Fanin != 7 || recs[0].Branch != "feat/x" {
		t.Fatalf("record = %+v", recs)
	}
	// missing plan → fail-open, nil, nothing written
	if err := runRouteLogRecordPlan(filepath.Join(t.TempDir(), "nope.md"), 0, &errb, func() (string, error) { return dir, nil }, git); err != nil {
		t.Fatalf("missing plan must be nil: %v", err)
	}
	recs, _ = routelog.LoadRecords(dir)
	if len(recs) != 1 {
		t.Fatalf("missing plan must not write: %d", len(recs))
	}
}

func TestRunRouteLogShow_EmptyAndSummary(t *testing.T) {
	var out, errb bytes.Buffer
	dir := filepath.Join(t.TempDir(), "rl")
	dirFn := func() (string, error) { return dir, nil }
	if err := runRouteLogShow(&out, &errb, false, "", dirFn); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "no routes logged yet") {
		t.Fatalf("empty: %q", out.String())
	}
	for _, r := range []routelog.Record{
		{TS: "2026-09-21T01:00:00Z", Site: "architect", Model: "fable", Gates: []string{"SHARED"}},
		{TS: "2026-09-21T02:00:00Z", Site: "manual", Model: "opus", Trigger: "manual"},
	} {
		if _, err := routelog.WriteRecord(dir, r); err != nil {
			t.Fatal(err)
		}
	}
	out.Reset()
	if err := runRouteLogShow(&out, &errb, false, "", dirFn); err != nil {
		t.Fatal(err)
	}
	for _, w := range []string{"routes", "architect", "manual", "SHARED"} {
		if !strings.Contains(out.String(), w) {
			t.Errorf("summary missing %q:\n%s", w, out.String())
		}
	}
	for _, w := range []string{"strong model sent", "architect changed", "changed_decision"} {
		if strings.Contains(out.String(), w) {
			t.Errorf("summary must not contain %q:\n%s", w, out.String())
		}
	}
	out.Reset()
	if err := runRouteLogShow(&out, &errb, true, "", dirFn); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(strings.TrimSpace(out.String()), "[") {
		t.Fatalf("--json must print an array: %q", out.String())
	}
}

func TestRouteLogCmd_ChildrenHidden(t *testing.T) {
	c := newRouteLogCmd()
	for _, name := range []string{"record", "record-plan"} {
		sub, _, err := c.Find([]string{name})
		if err != nil || sub == nil || !sub.Hidden {
			t.Fatalf("%s must exist and be hidden: %v", name, err)
		}
	}
}

func TestRouteLogCmd_RecordPlanWithoutFlag_ExitsZero(t *testing.T) {
	c := newRouteLogCmd()
	var errb bytes.Buffer
	c.SetArgs([]string{"record-plan"})
	c.SetErr(&errb)
	c.SetOut(&errb)
	if err := c.Execute(); err != nil {
		t.Fatalf("record-plan without --plan must stay best-effort (exit 0): %v", err)
	}
}

func TestRuntimeFromEnvOrder(t *testing.T) {
	env := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
	if got := runtimeFromEnv(env(map[string]string{"CODEX_THREAD_ID": "1", "CLAUDECODE": "1"})); got != "codex" {
		t.Fatalf("both set: got %q, want codex", got)
	}
	if got := runtimeFromEnv(env(map[string]string{"CLAUDECODE": "1"})); got != "claude" {
		t.Fatalf("claude only: got %q, want claude", got)
	}
	if got := runtimeFromEnv(env(nil)); got != "unknown" {
		t.Fatalf("none: got %q, want unknown", got)
	}
}

func TestRouteLogRecordRuntimeOverridesPayload(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "rl")
	dirFn := func() (string, error) { return dir, nil }
	getenv := func(k string) string {
		if k == "CODEX_THREAD_ID" {
			return "1"
		}
		return ""
	}
	var errb bytes.Buffer
	if err := runRouteLogRecord(strings.NewReader(`{"ts":"2026-10-12T01:00:00Z","site":"x","runtime":"claude"}`), &errb, dirFn, getenv); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		b, _ := os.ReadFile(filepath.Join(dir, e.Name()))
		if !strings.Contains(string(b), `"runtime":"codex"`) {
			t.Fatalf("record = %s, want runtime codex", b)
		}
		return
	}
	t.Fatal("no record written")
}
