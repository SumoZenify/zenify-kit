package review

import (
	"errors"
	"os"
	"os/exec"
	"testing"
	"time"
)

const fixRoot = "/private/tmp/claude-501/codexreview"

func fakeRead(string) ([]byte, error) {
	return []byte("l1\nl2\nl3\nREAL LINE 4\nl5\nl6\n"), nil
}

func TestParseCodexReviewFixtureB(t *testing.T) {
	out, err := os.ReadFile("testdata/codex_review_b.out")
	if err != nil {
		t.Fatal(err)
	}
	fs, _ := ParseCodexReview(out, fixRoot, fakeRead)
	if len(fs) != 3 {
		t.Fatalf("findings = %d", len(fs))
	}
	for _, f := range fs {
		if f.Severity != "HIGH" || f.File != "src/users.js" {
			t.Errorf("bad finding %+v", f)
		}
		if f.Issue == "" || f.Fix != "" {
			t.Errorf("issue/fix wrong %+v", f)
		}
	}
	if fs[2].Evidence != "l1" || fs[1].Evidence != "l6" {
		t.Errorf("evidence 1/2: %q %q", fs[1].Evidence, fs[2].Evidence)
	}
	if fs[0].Line != "4" || fs[0].Evidence != "REAL LINE 4" {
		t.Errorf("finding0 line/evidence: %+v", fs[0])
	}
	if fs[0].Title != "Parameterize the user-supplied search term" {
		t.Errorf("title %q", fs[0].Title)
	}
	for _, f := range fs {
		if f.Dimension != "bugs" {
			t.Errorf("dimension %q for %q", f.Dimension, f.Title)
		}
	}
}

func TestParseCodexReviewSecurityDimensionAndSeverities(t *testing.T) {
	in := "- [P0] SQL injection in handler — /r/a.go:3\n  body\n- [P3] nit — /r/b.go:1-2\n"
	fs, _ := ParseCodexReview([]byte(in), "/r", func(string) ([]byte, error) { return nil, errors.New("x") })
	if len(fs) != 2 || fs[0].Severity != "CRITICAL" || fs[0].Dimension != "security" || fs[0].Line != "3" ||
		fs[1].Severity != "LOW" || fs[1].Line != "1" || fs[0].Evidence != "" {
		t.Fatalf("got %+v", fs)
	}
}

func TestParseCodexReviewProseOnly(t *testing.T) {
	out, err := os.ReadFile("testdata/codex_review_prose.out")
	if err != nil {
		t.Fatal(err)
	}
	fs, prose := ParseCodexReview(out, fixRoot, fakeRead)
	if len(fs) != 0 || prose == 0 {
		t.Fatalf("findings=%d prose=%d", len(fs), prose)
	}
}

func TestParseCodexReviewPathOutsideRepo(t *testing.T) {
	in := "- [P2] Something — /elsewhere/x.js:7-9\n  body\n"
	fs, _ := ParseCodexReview([]byte(in), "/repo", fakeRead)
	if len(fs) != 1 || fs[0].File != "/elsewhere/x.js" || fs[0].Severity != "MEDIUM" {
		t.Fatalf("got %+v", fs)
	}
}

func TestCodexReasonMatrix(t *testing.T) {
	cases := []struct {
		err      error
		stderr   string
		timeout  time.Duration
		timedOut bool
		want     string
	}{
		{&exec.Error{Name: "codex", Err: exec.ErrNotFound}, "", 0, false, "codex không có trên PATH"}, //znf:allow-lang
		{errors.New("exit status 1"), "Please login first", 0, false, "codex chưa đăng nhập"},         //znf:allow-lang
		{errors.New("exit status 1"), "HTTP 401 Unauthorized", 0, false, "codex chưa đăng nhập"},      //znf:allow-lang
		{errors.New("exit status 1"), "Rate limit exceeded", 0, false, "rate limit"},
		{errors.New("exit status 1"), "got 429", 0, false, "rate limit"},
		{errors.New("signal: killed"), "", 10 * time.Millisecond, true, "timeout 10ms"},
		{fakeExit(3), "boom", 0, false, "codex review exit 3"},
	}
	for _, c := range cases {
		if got := CodexReason(c.err, c.stderr, c.timeout, c.timedOut); got != c.want {
			t.Errorf("CodexReason(%v,%q) = %q want %q", c.err, c.stderr, got, c.want)
		}
	}
}

type fakeExit int

func (f fakeExit) Error() string { return "exit status" }
func (f fakeExit) ExitCode() int { return int(f) }
