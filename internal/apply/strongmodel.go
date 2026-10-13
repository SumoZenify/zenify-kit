// internal/apply/strongmodel.go
package apply

import (
	"path/filepath"
)

// StrongModelEnv is the retired env var the kit used to seed for its own model
// routing. The kit no longer sets it; RemoveStrongModelEnv cleans it up.
const StrongModelEnv = "ZNF_STRONG_MODEL"

// RemoveStrongModelEnv removes the retired key at any value (the kit no longer
// reads it, so a hand-set value is dead too).
func RemoveStrongModelEnv(home string, dryRun bool) (bool, error) {
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
	_, ok := env[StrongModelEnv]
	if !ok {
		return false, nil
	}
	if dryRun {
		return true, nil
	}
	delete(env, StrongModelEnv)
	return true, writeEnv(path, root, env, mode)
}
