package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

type fakeExit int

func (f fakeExit) Error() string { return "exit status" }
func (f fakeExit) ExitCode() int { return int(f) }

func runnerReturning(out, errOut string, err error) func(context.Context, string, []string) ([]byte, []byte, error) {
	return func(context.Context, string, []string) ([]byte, []byte, error) {
		return []byte(out), []byte(errOut), err
	}
}

func TestReviewCodexFixtureB(t *testing.T) {
	fx, err := os.ReadFile("../review/testdata/codex_review_b.out")
	if err != nil {
		t.Fatal(err)
	}
	var gotName string
	var gotArgs []string
	runner := func(_ context.Context, name string, args []string) ([]byte, []byte, error) {
		gotName, gotArgs = name, args
		return fx, nil, nil
	}
	var so, se bytes.Buffer
	code := runReviewCodex(reviewCodexOpts{Base: "origin/staging", Timeout: time.Minute, RepoRoot: "/tmp/claude-501/codexreview"}, runner, &so, &se)
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, se.String())
	}
	var res struct {
		Findings []map[string]any `json:"findings"`
	}
	if err := json.Unmarshal(so.Bytes(), &res); err != nil || len(res.Findings) != 3 {
		t.Fatalf("stdout %q err %v", so.String(), err)
	}
	if gotName != "codex" || len(gotArgs) != 4 || gotArgs[0] != "review" || gotArgs[1] != "-c" ||
		gotArgs[2] != "model_reasoning_effort=medium" || !strings.HasPrefix(gotArgs[3], "Review ONLY the changes of this repository relative to origin/staging, including uncommitted working-tree changes (staged and unstaged).") {
		t.Fatalf("args %q %q", gotName, gotArgs)
	}
}

func TestReviewCodexRateLimit(t *testing.T) {
	var so, se bytes.Buffer
	code := runReviewCodex(reviewCodexOpts{Base: "x", Timeout: time.Minute}, runnerReturning("", "429 rate limit", fakeExit(1)), &so, &se)
	if code != 1 || strings.TrimSpace(se.String()) != "rate limit" || so.Len() != 0 {
		t.Fatalf("code %d stderr %q stdout %q", code, se.String(), so.String())
	}
}

func TestReviewCodexTimeout(t *testing.T) {
	runner := func(ctx context.Context, _ string, _ []string) ([]byte, []byte, error) {
		<-ctx.Done()
		return nil, nil, errors.New("signal: killed")
	}
	var so, se bytes.Buffer
	code := runReviewCodex(reviewCodexOpts{Base: "x", Timeout: 10 * time.Millisecond}, runner, &so, &se)
	if code != 1 || strings.TrimSpace(se.String()) != "timeout 10ms" {
		t.Fatalf("code %d stderr %q", code, se.String())
	}
}

func TestReviewCodexProseOnly(t *testing.T) {
	var so, se bytes.Buffer
	code := runReviewCodex(reviewCodexOpts{Base: "x", Timeout: time.Minute}, runnerReturning("All good.\nNothing to report.\n", "", nil), &so, &se)
	if code != 0 || !strings.Contains(se.String(), "0 finding parse được") || strings.TrimSpace(so.String()) != `{"findings":[]}` { //znf:allow-lang
		t.Fatalf("code %d stderr %q stdout %q", code, se.String(), so.String())
	}
}

func TestReviewCodexPromptWithPack(t *testing.T) {
	pack := t.TempDir() + "/pack.md"
	if err := os.WriteFile(pack, []byte("add user search"), 0o600); err != nil {
		t.Fatal(err)
	}
	var prompt string
	runner := func(_ context.Context, _ string, args []string) ([]byte, []byte, error) {
		prompt = args[len(args)-1]
		return []byte("ok\n"), nil, nil
	}
	var so, se bytes.Buffer
	if code := runReviewCodex(reviewCodexOpts{Base: "b", Pack: pack, Timeout: time.Minute}, runner, &so, &se); code != 0 {
		t.Fatalf("exit %d", code)
	}
	want := "\nIntent:\nadd user search\nReport every finding as exactly one line:"
	if !strings.Contains(prompt, want) {
		t.Fatalf("prompt %q lacks %q", prompt, want)
	}
}

// Standalone /review passes Base "HEAD" and its scope is `git diff HEAD` (the working tree),
// so the prompt must not collapse to the always-empty HEAD..HEAD range.
func TestReviewCodexPromptKeepsWorkingTreeScope(t *testing.T) {
	var prompt string
	runner := func(_ context.Context, _ string, args []string) ([]byte, []byte, error) {
		prompt = args[len(args)-1]
		return []byte("ok\n"), nil, nil
	}
	var so, se bytes.Buffer
	if code := runReviewCodex(reviewCodexOpts{Base: "HEAD", Timeout: time.Minute}, runner, &so, &se); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if strings.Contains(prompt, "HEAD..HEAD") {
		t.Fatalf("prompt %q holds the empty HEAD..HEAD range", prompt)
	}
	want := "Review ONLY the changes of this repository relative to HEAD, including uncommitted working-tree changes (staged and unstaged)."
	if !strings.HasPrefix(prompt, want) {
		t.Fatalf("prompt %q lacks %q", prompt, want)
	}
}
