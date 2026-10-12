package review

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// CodexFinding is a finding parsed from `codex review` output.
type CodexFinding = Finding

var codexBullet = regexp.MustCompile(`^- \[P([0-3])\] (.+?) — (.+?):(\d+)(?:-(\d+))?$`)

var codexSeverity = map[string]string{"0": "CRITICAL", "1": "HIGH", "2": "MEDIUM", "3": "LOW"}

// ParseCodexReview turns the prose output of `codex review` into findings.
// proseLines counts the non-empty lines that belong to no finding.
func ParseCodexReview(out []byte, repoRoot string, readFile func(string) ([]byte, error)) (findings []Finding, proseLines int) {
	var cur *Finding
	var issue []string
	flush := func() {
		if cur != nil {
			cur.Issue = strings.Join(issue, " ")
			findings = append(findings, *cur)
		}
		cur, issue = nil, nil
	}
	for _, raw := range strings.Split(string(out), "\n") {
		line := strings.TrimRight(raw, "\r")
		if m := codexBullet.FindStringSubmatch(line); m != nil {
			flush()
			cur = &Finding{
				Severity:  codexSeverity[m[1]],
				Title:     m[2],
				File:      relToRoot(repoRoot, m[3]),
				Line:      m[4],
				Dimension: codexDimension(m[2]),
				Evidence:  lineAt(readFile, m[3], m[4]),
			}
			continue
		}
		if strings.TrimSpace(line) == "" {
			flush()
			continue
		}
		if cur != nil {
			issue = append(issue, strings.TrimSpace(line))
		} else {
			proseLines++
		}
	}
	flush()
	return findings, proseLines
}

// relToRoot makes abs relative to root when it lies inside it; the root is also
// tried with symlinks resolved (macOS prints /private/tmp for /tmp).
func relToRoot(root, abs string) string {
	roots := []string{root}
	if r, err := filepath.EvalSymlinks(root); err == nil && r != root {
		roots = append(roots, r)
	}
	for _, r := range roots {
		rel, err := filepath.Rel(r, abs)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return rel
		}
	}
	return abs
}

func lineAt(readFile func(string) ([]byte, error), path, line string) string {
	n, err := strconv.Atoi(line)
	if err != nil || n < 1 {
		return ""
	}
	data, err := readFile(path)
	if err != nil {
		return ""
	}
	lines := strings.Split(string(data), "\n")
	if n > len(lines) {
		return ""
	}
	return strings.TrimRight(lines[n-1], "\r")
}

func codexDimension(title string) string {
	t := strings.ToLower(title)
	for _, k := range []string{"security", "injection", "auth"} {
		if strings.Contains(t, k) {
			return "security"
		}
	}
	return "bugs"
}

// CodexReason names why a `codex review` run failed, in one short phrase.
func CodexReason(err error, stderr string, timeout time.Duration, timedOut bool) string {
	if errors.Is(err, exec.ErrNotFound) {
		return "codex không có trên PATH" //znf:allow-lang
	}
	if timedOut {
		return "timeout " + timeout.String()
	}
	low := strings.ToLower(stderr)
	if strings.Contains(low, "login") || strings.Contains(low, "401") || strings.Contains(low, "unauthorized") {
		return "codex chưa đăng nhập" //znf:allow-lang
	}
	if strings.Contains(low, "rate limit") || strings.Contains(low, "429") {
		return "rate limit"
	}
	code := -1
	var ee interface{ ExitCode() int }
	if errors.As(err, &ee) {
		code = ee.ExitCode()
	}
	return fmt.Sprintf("codex review exit %d", code)
}
