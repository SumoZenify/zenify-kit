package dbread

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/ZenifyAIContactCenter/zenify-kit/internal/managed"
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

// SetupInput contains one profile's metadata and credential values collected
// by the terminal wizard. Credential values are written only to
// settings.local.json and are never included in db-read.json.
type SetupInput struct {
	Name        string
	Profile     Profile
	Credentials map[string]string
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
	if err := validateProfile(name, p); err != nil {
		return Profile{}, err
	}
	return p, nil
}

func validateProfile(name string, p Profile) error {
	if p.Mongo == nil && p.MySQL == nil {
		return fmt.Errorf("db-read profile %q has no database", name)
	}
	if p.Mongo != nil && (!envKeyPattern.MatchString(p.Mongo.URIEnv) || p.Mongo.Database == "") {
		return fmt.Errorf("db-read profile %q has invalid Mongo settings", name)
	}
	if p.MySQL != nil && (!envKeyPattern.MatchString(p.MySQL.HostEnv) || !envKeyPattern.MatchString(p.MySQL.UserEnv) || !envKeyPattern.MatchString(p.MySQL.PasswordEnv) || p.MySQL.Database == "") {
		return fmt.Errorf("db-read profile %q has invalid MySQL settings", name)
	}
	if p.MySQL != nil && p.MySQL.PortEnv != "" && !envKeyPattern.MatchString(p.MySQL.PortEnv) {
		return fmt.Errorf("db-read profile %q has invalid MySQL port key", name)
	}
	return nil
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

// SaveProfile stores one profile's metadata and credentials. It preserves all
// other profiles and all unrelated settings. Empty credential inputs keep an
// existing value; if no value exists, the write is rejected before either
// file is changed.
func SaveProfile(settingsPath string, input SetupInput) (string, error) {
	if settingsPath == "" {
		return "", fmt.Errorf("no workspace: run zenify up first")
	}
	if !profileNamePattern.MatchString(input.Name) {
		return "", fmt.Errorf("invalid db-read profile name %q", input.Name)
	}
	if err := validateProfile(input.Name, input.Profile); err != nil {
		return "", err
	}

	configPath := profileConfigPath(settingsPath)
	cfg := ProfileConfig{Profiles: map[string]Profile{}}
	if b, err := os.ReadFile(configPath); err == nil { //nolint:gosec // path is derived from the resolved workspace
		if err := json.Unmarshal(b, &cfg); err != nil {
			return "", fmt.Errorf("invalid db-read profile config, left as-is: %w", err)
		}
		if cfg.Profiles == nil {
			cfg.Profiles = map[string]Profile{}
		}
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("cannot read db-read profiles at %s: %w", configPath, err)
	}

	settingsRoot, settingsEnv, err := readSettingsDocument(settingsPath)
	if err != nil {
		return "", err
	}
	for _, key := range credentialKeys(input.Profile) {
		if value := input.Credentials[key]; value != "" {
			settingsEnv[key] = value
		}
	}
	missing := missingCredentialKeys(input.Profile, settingsEnv)
	if len(missing) > 0 {
		return "", fmt.Errorf("missing credential values for: %s", strings.Join(missing, ", "))
	}

	cfg.Profiles[input.Name] = input.Profile
	configData, err := encodeJSON(cfg)
	if err != nil {
		return "", err
	}
	settingsRoot["env"] = settingsEnv
	settingsData, err := encodeJSON(settingsRoot)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o750); err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(configPath), 0o750); err != nil {
		return "", err
	}
	if err := managed.WriteFileAtomicMode(settingsPath, settingsData, 0o600); err != nil {
		return "", err
	}
	if err := managed.WriteFileAtomic(configPath, configData); err != nil {
		return "", err
	}
	return configPath, nil
}

func readSettingsDocument(path string) (map[string]any, map[string]any, error) {
	root := map[string]any{}
	b, err := os.ReadFile(path) //nolint:gosec // path is derived from the resolved workspace
	if err == nil {
		if err := json.Unmarshal(b, &root); err != nil {
			return nil, nil, fmt.Errorf("settings %s is not valid JSON, left as-is: %w", path, err)
		}
	} else if !os.IsNotExist(err) {
		return nil, nil, err
	}
	if root == nil {
		root = map[string]any{}
	}
	env, ok := root["env"].(map[string]any)
	if !ok {
		if _, present := root["env"]; present {
			return nil, nil, fmt.Errorf("settings %s has \"env\" that isn't an object, left as-is", path)
		}
		env = map[string]any{}
	}
	return root, env, nil
}

func missingCredentialKeys(profile Profile, env map[string]any) []string {
	missing := make([]string, 0)
	for _, key := range requiredCredentialKeys(profile) {
		if value, ok := env[key].(string); !ok || value == "" {
			missing = append(missing, key)
		}
	}
	sort.Strings(missing)
	return missing
}

func credentialKeys(profile Profile) []string {
	keys := make([]string, 0, 5)
	if profile.Mongo != nil {
		keys = append(keys, profile.Mongo.URIEnv)
	}
	if profile.MySQL != nil {
		keys = append(keys, profile.MySQL.HostEnv, profile.MySQL.UserEnv, profile.MySQL.PasswordEnv)
		if profile.MySQL.PortEnv != "" {
			keys = append(keys, profile.MySQL.PortEnv)
		}
	}
	return keys
}

func requiredCredentialKeys(profile Profile) []string {
	keys := make([]string, 0, 4)
	if profile.Mongo != nil {
		keys = append(keys, profile.Mongo.URIEnv)
	}
	if profile.MySQL != nil {
		keys = append(keys, profile.MySQL.HostEnv, profile.MySQL.UserEnv, profile.MySQL.PasswordEnv)
	}
	return keys
}

func encodeJSON(value any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(value); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
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
