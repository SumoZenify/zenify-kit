package cli

import (
	"bytes"
	"testing"
)

func TestDecideReviewerMatrix(t *testing.T) {
	for _, thread := range []string{"", "1"} {
		for _, zr := range []string{"agent", "codex"} {
			for _, onPath := range []bool{true, false} {
				env := map[string]string{"CODEX_THREAD_ID": thread, "ZNF_REVIEWER": zr}
				got, reason := decideReviewer(func(k string) string { return env[k] }, onPath)
				want, wantReason := "agent", ""
				if (thread != "" || zr == "codex") && onPath {
					want = "codex"
				} else if !onPath && (thread != "" || zr == "codex") {
					wantReason = "codex không có trên PATH" //znf:allow-lang
				} else {
					wantReason = "ZNF_REVIEWER=agent"
				}
				if got != want || reason != wantReason {
					t.Errorf("thread=%q zr=%q onPath=%v: got (%q,%q) want (%q,%q)", thread, zr, onPath, got, reason, want, wantReason)
				}
			}
		}
	}
}

func TestReviewerCmdPrints(t *testing.T) {
	var out bytes.Buffer
	printReviewer(&out, "agent", "ZNF_REVIEWER=agent")
	if out.String() != "agent (ZNF_REVIEWER=agent)\n" {
		t.Fatalf("got %q", out.String())
	}
	out.Reset()
	printReviewer(&out, "codex", "")
	if out.String() != "codex\n" {
		t.Fatalf("got %q", out.String())
	}
}
