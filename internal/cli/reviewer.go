package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/ZenifyAIContactCenter/zenify-kit/internal/apply"
	"github.com/spf13/cobra"
)

// decideReviewer picks the review engine: codex only when the session asks for
// it (running inside Codex, or ZNF_REVIEWER=codex) and the binary is on PATH.
func decideReviewer(getenv func(string) string, onPath bool) (engine, reason string) {
	wantsCodex := getenv("CODEX_THREAD_ID") != "" || getenv(apply.ReviewerEnv) == "codex"
	switch {
	case wantsCodex && onPath:
		return "codex", ""
	case wantsCodex:
		return "agent", "codex không có trên PATH" //znf:allow-lang
	default:
		return "agent", "ZNF_REVIEWER=agent"
	}
}

func printReviewer(w io.Writer, engine, reason string) {
	if reason == "" {
		fmt.Fprintln(w, engine)
		return
	}
	fmt.Fprintf(w, "%s (%s)\n", engine, reason)
}

func newReviewerCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "reviewer",
		Short: "In engine review được chọn (agent hoặc codex) và lý do", //znf:allow-lang
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			engine, reason := decideReviewer(os.Getenv, codexOnPath())
			printReviewer(cmd.OutOrStdout(), engine, reason)
			return nil
		},
	}
}
