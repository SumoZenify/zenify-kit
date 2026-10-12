package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/ZenifyAIContactCenter/zenify-kit/internal/review"
	"github.com/spf13/cobra"
)

type reviewCodexOpts struct {
	Base     string
	Pack     string // path to an intent file; empty = none
	Timeout  time.Duration
	RepoRoot string // empty = cwd
}

// codexWaitDelay bounds how long Run waits for pipes to close after ctx kills codex.
const codexWaitDelay = 5 * time.Second

type codexRunner func(ctx context.Context, name string, args []string) (stdout, stderr []byte, err error)

func execCodexRunner(ctx context.Context, name string, args []string) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // G204 -- name is the fixed codex command chosen by the kit and args are built by the kit, not user input; Stdin nil = /dev/null (closed)
	cmd.WaitDelay = codexWaitDelay                 // a child holding the pipes must not outlive --timeout
	var so, se bytes.Buffer
	cmd.Stdout, cmd.Stderr = &so, &se
	err := cmd.Run()
	return so.Bytes(), se.Bytes(), err
}

func codexReviewPrompt(base, pack string) string {
	p := "Review ONLY the changes of this repository relative to " + base + ", including uncommitted working-tree changes (staged and unstaged)."
	if pack != "" {
		p += "\nIntent:\n" + pack
	}
	return p + "\nReport every finding as exactly one line: - [P<n>] <title> — <absolute path>:<start line>-<end line>, followed by one short paragraph."
}

// runReviewCodex runs `codex review` and prints the kit's findings JSON on stdout.
// Exit 0 on success (even with 0 findings), 1 with a one-line reason on failure.
func runReviewCodex(opts reviewCodexOpts, runner codexRunner, stdout, stderr io.Writer) int {
	pack := ""
	if opts.Pack != "" {
		b, err := os.ReadFile(opts.Pack)
		if err != nil {
			fmt.Fprintln(stderr, "review-codex: không đọc được --pack:", err) //znf:allow-lang
			return 1
		}
		pack = string(b)
	}
	root := opts.RepoRoot
	if root == "" {
		root, _ = os.Getwd()
	}
	ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
	defer cancel()
	args := []string{"review", "-c", "model_reasoning_effort=medium", codexReviewPrompt(opts.Base, pack)}
	out, errOut, err := runner(ctx, "codex", args)
	if err != nil {
		timedOut := errors.Is(ctx.Err(), context.DeadlineExceeded)
		fmt.Fprintln(stderr, review.CodexReason(err, string(errOut), opts.Timeout, timedOut))
		return 1
	}
	findings, prose := review.ParseCodexReview(out, root, os.ReadFile)
	if findings == nil {
		findings = []review.Finding{}
	}
	if len(findings) == 0 && len(bytes.TrimSpace(out)) > 0 {
		fmt.Fprintf(stderr, "codex review: 0 finding parse được, %d dòng prose\n", prose) //znf:allow-lang
	}
	enc := json.NewEncoder(stdout)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(struct {
		Findings []review.Finding `json:"findings"`
	}{findings}); err != nil {
		fmt.Fprintln(stderr, "review-codex: marshal lỗi:", err) //znf:allow-lang
		return 1
	}
	return 0
}

func newReviewCodexCmd() *cobra.Command {
	var opts reviewCodexOpts
	cmd := &cobra.Command{
		Use:    "review-codex",
		Short:  "Chạy codex review rồi in findings JSON của kit (adapter, seam review engine)", //znf:allow-lang
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if code := runReviewCodex(opts, execCodexRunner, cmd.OutOrStdout(), cmd.ErrOrStderr()); code != 0 {
				os.Exit(code) // the reason is already on stderr; main would add a second line
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&opts.Base, "base", "", "base ref of the diff to review (required)")
	cmd.Flags().StringVar(&opts.Pack, "pack", "", "file holding the intent to pass to codex")
	cmd.Flags().DurationVar(&opts.Timeout, "timeout", 10*time.Minute, "give up after this long")
	_ = cmd.MarkFlagRequired("base")
	return cmd
}
