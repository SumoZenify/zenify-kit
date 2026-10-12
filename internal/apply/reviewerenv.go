// internal/apply/reviewerenv.go
package apply

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
)

// ReviewerEnv is the env var `zenify reviewer` reads to pick the review
// engine. Values: "agent" (the Claude reviewer subagents) or "codex".
const ReviewerEnv = "ZNF_REVIEWER"

// ReviewerDefault is what the kit seeds: agent.
const ReviewerDefault = "agent"

// EnsureReviewerEnv sets env.ZNF_REVIEWER in <home>/.claude/settings.json
// only when the key is absent (same contract as EnsureStrongModelEnv).
func EnsureReviewerEnv(home string, dryRun bool) (bool, error) {
	path := filepath.Join(home, ".claude", "settings.json")
	root, mode, err := readSettingsRoot(path)
	if err != nil {
		return false, err
	}
	env, err := envOf(root)
	if err != nil {
		return false, err
	}
	if _, ok := env[ReviewerEnv]; ok {
		return false, nil
	}
	if dryRun {
		return true, nil
	}
	val, _ := json.Marshal(ReviewerDefault)
	env[ReviewerEnv] = json.RawMessage(val)
	return true, writeEnv(path, root, env, mode)
}

// SetReviewerEnv overwrites env.ZNF_REVIEWER with value (agent or codex).
func SetReviewerEnv(home, value string, dryRun bool) error {
	if value != "agent" && value != "codex" {
		return fmt.Errorf("reviewer %q: want agent or codex", value)
	}
	path := filepath.Join(home, ".claude", "settings.json")
	root, mode, err := readSettingsRoot(path)
	if err != nil {
		return err
	}
	env, err := envOf(root)
	if err != nil {
		return err
	}
	if dryRun {
		return nil
	}
	val, _ := json.Marshal(value)
	env[ReviewerEnv] = json.RawMessage(val)
	return writeEnv(path, root, env, mode)
}

// RemoveReviewerEnv removes the key only while it still holds the kit
// default, so a value the user changed stays.
func RemoveReviewerEnv(home string, dryRun bool) (bool, error) {
	path := filepath.Join(home, ".claude", "settings.json")
	root, mode, err := readSettingsRoot(path)
	if err != nil {
		return false, err
	}
	if root == nil {
		return false, nil
	}
	env, err := envOf(root)
	if err != nil {
		return false, err
	}
	cur, ok := env[ReviewerEnv]
	if !ok {
		return false, nil
	}
	want, _ := json.Marshal(ReviewerDefault)
	if !bytes.Equal(bytes.TrimSpace(cur), want) {
		return false, nil
	}
	if dryRun {
		return true, nil
	}
	delete(env, ReviewerEnv)
	return true, writeEnv(path, root, env, mode)
}
