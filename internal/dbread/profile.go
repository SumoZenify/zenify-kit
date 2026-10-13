package dbread

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// ProfileConfig is workspace-owned metadata. It contains environment variable
// names, never credential values. The selected profile never falls back to
// another target when a key or connection is missing.
type ProfileConfig struct {
	Profiles map[string]Profile `json:"profiles"`
}

type Profile struct {
	Mongo *MongoProfile `json:"mongo,omitempty"`
	MySQL *MySQLProfile `json:"mysql,omitempty"`
}

type MongoProfile struct {
	URIEnv   string `json:"uriEnv"`
	Database string `json:"database"`
}

type MySQLProfile struct {
	HostEnv     string `json:"hostEnv"`
	PortEnv     string `json:"portEnv"`
	UserEnv     string `json:"userEnv"`
	PasswordEnv string `json:"passwordEnv"`
	Database    string `json:"database"`
}

var (
	envKeyPattern      = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	profileNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)
)

func profileConfigPath(settingsPath string) string {
	if settingsPath == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(filepath.Dir(settingsPath)), ".zenify", "db-read.json")
}

func loadProfile(path, name string) (Profile, error) {
	if !profileNamePattern.MatchString(name) {
		return Profile{}, fmt.Errorf("invalid db-read profile name %q", name)
	}
	cfg, err := loadProfileConfig(path)
	if err != nil {
		return Profile{}, err
	}
	p, ok := cfg.Profiles[name]
	if !ok {
		return Profile{}, fmt.Errorf("db-read profile %q is not configured", name)
	}
	if p.Mongo == nil && p.MySQL == nil {
		return Profile{}, fmt.Errorf("db-read profile %q has no database", name)
	}
	if p.Mongo != nil && (!envKeyPattern.MatchString(p.Mongo.URIEnv) || p.Mongo.Database == "") {
		return Profile{}, fmt.Errorf("db-read profile %q has invalid Mongo settings", name)
	}
	if p.MySQL != nil && (!envKeyPattern.MatchString(p.MySQL.HostEnv) || !envKeyPattern.MatchString(p.MySQL.UserEnv) || !envKeyPattern.MatchString(p.MySQL.PasswordEnv) || p.MySQL.Database == "") {
		return Profile{}, fmt.Errorf("db-read profile %q has invalid MySQL settings", name)
	}
	if p.MySQL != nil && p.MySQL.PortEnv != "" && !envKeyPattern.MatchString(p.MySQL.PortEnv) {
		return Profile{}, fmt.Errorf("db-read profile %q has invalid MySQL port key", name)
	}
	return p, nil
}

func loadProfileConfig(path string) (ProfileConfig, error) {
	if path == "" {
		return ProfileConfig{}, fmt.Errorf("no workspace: run zenify up first")
	}
	b, err := os.ReadFile(path) //nolint:gosec // path is derived from the resolved workspace
	if err != nil {
		return ProfileConfig{}, fmt.Errorf("cannot read db-read profiles at %s: %w", path, err)
	}
	var cfg ProfileConfig
	if err := json.Unmarshal(b, &cfg); err != nil {
		return ProfileConfig{}, fmt.Errorf("invalid db-read profile config: %w", err)
	}
	return cfg, nil
}

func credentialEnvClears(path string) []string {
	cfg, err := loadProfileConfig(path)
	if err != nil {
		return nil
	}
	keys := map[string]struct{}{}
	add := func(key string) {
		if envKeyPattern.MatchString(key) {
			keys[key] = struct{}{}
		}
	}
	for _, p := range cfg.Profiles {
		if p.Mongo != nil {
			add(p.Mongo.URIEnv)
		}
		if p.MySQL != nil {
			add(p.MySQL.HostEnv)
			add(p.MySQL.PortEnv)
			add(p.MySQL.UserEnv)
			add(p.MySQL.PasswordEnv)
		}
	}
	names := make([]string, 0, len(keys))
	for key := range keys {
		names = append(names, key)
	}
	sort.Strings(names)
	clears := make([]string, 0, len(names))
	for _, key := range names {
		clears = append(clears, key+"=")
	}
	return clears
}

func loadSettingsEnv(settingsPath string) map[string]string {
	values := map[string]string{}
	b, err := os.ReadFile(settingsPath) //nolint:gosec // path is derived from the resolved workspace
	if err != nil {
		return values
	}
	var doc struct {
		Env map[string]string `json:"env"`
	}
	if json.Unmarshal(b, &doc) == nil {
		return doc.Env
	}
	return values
}

func resolveEnv(getenv func(string) string, settings map[string]string, key string) string {
	if key == "" {
		return ""
	}
	if value := getenv(key); value != "" {
		return value
	}
	return settings[key]
}

// CheckProfile reports whether a profile and all of its configured
// credentials are available. It returns key names and presence only.
func CheckProfile(getenv func(string) string, settingsPath, name string) (bool, string) {
	p, err := loadProfile(profileConfigPath(settingsPath), name)
	if err != nil {
		return false, err.Error()
	}
	settings := loadSettingsEnv(settingsPath)
	var states []string
	ok := true
	check := func(key string) {
		state := "present"
		if resolveEnv(getenv, settings, key) == "" {
			state = "absent"
			ok = false
		}
		states = append(states, key+"="+state)
	}
	if p.Mongo != nil {
		check(p.Mongo.URIEnv)
	}
	if p.MySQL != nil {
		check(p.MySQL.HostEnv)
		check(p.MySQL.UserEnv)
		check(p.MySQL.PasswordEnv)
	}
	return ok, "profile=" + name + " " + strings.Join(states, " ")
}
