package apply

import (
	"fmt"
	"os"
	"path/filepath"
)

// LinkState is the outcome of EnsureCodexSkillsLink.
type LinkState int

const (
	LinkCreated   LinkState = iota // link did not exist and was (or would be) created
	LinkUnchanged                  // link already points at the kit plugin
	LinkRewritten                  // a symlink pointed elsewhere and was (or would be) repointed
	LinkRealDir                    // a real file/dir sits at the link path; left alone
)

func codexSkillsPaths(home string) (link, target string) {
	return filepath.Join(home, ".agents", "skills", "znf"), filepath.Join(home, ".claude", "skills", "znf")
}

// EnsureCodexSkillsLink makes <home>/.agents/skills/znf an absolute symlink to
// <home>/.claude/skills/znf so Codex discovers the kit plugin's skills.
func EnsureCodexSkillsLink(home string, dryRun bool) (LinkState, error) {
	link, target := codexSkillsPaths(home)
	state := LinkCreated
	fi, err := os.Lstat(link)
	switch {
	case err == nil && fi.Mode()&os.ModeSymlink == 0:
		return LinkRealDir, nil
	case err == nil:
		cur, rerr := os.Readlink(link)
		if rerr != nil {
			return LinkCreated, fmt.Errorf("readlink %s: %w", link, rerr)
		}
		if cur == target {
			return LinkUnchanged, nil
		}
		state = LinkRewritten
	case !os.IsNotExist(err):
		return LinkCreated, fmt.Errorf("lstat %s: %w", link, err)
	}
	if dryRun {
		return state, nil
	}
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil { //nolint:gosec // G301 -- skills dir must be readable by codex
		return state, fmt.Errorf("mkdir %s: %w", filepath.Dir(link), err)
	}
	if state == LinkRewritten {
		if err := os.Remove(link); err != nil {
			return state, fmt.Errorf("remove %s: %w", link, err)
		}
	}
	if err := os.Symlink(target, link); err != nil {
		return state, fmt.Errorf("symlink %s: %w", link, err)
	}
	return state, nil
}

// RemoveCodexSkillsLink removes the link only when it is a symlink pointing at
// the kit plugin; anything else is left alone and reported as false.
func RemoveCodexSkillsLink(home string, dryRun bool) (bool, error) {
	link, target := codexSkillsPaths(home)
	fi, err := os.Lstat(link)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("lstat %s: %w", link, err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		return false, nil
	}
	if cur, err := os.Readlink(link); err != nil || cur != target {
		return false, nil
	}
	if dryRun {
		return true, nil
	}
	if err := os.Remove(link); err != nil {
		return false, fmt.Errorf("remove %s: %w", link, err)
	}
	return true, nil
}
