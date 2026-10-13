package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func doctorSettingsPath(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	settings := filepath.Join(root, ".claude", "settings.local.json")
	config := filepath.Join(root, ".zenify", "db-read.json")
	if err := os.MkdirAll(filepath.Dir(config), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config, []byte(`{"profiles":{"staging":{"mongo":{"uriEnv":"MONGO_URL","database":"app_staging"}}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	return settings
}

func TestSecretPresenceNamesOnlyNeverLeaksValue(t *testing.T) {
	getenv := func(k string) string {
		if k == "MONGO_URL" {
			return "mongodb://secretuser:secretpass@10.0.0.1:27017"
		}
		return ""
	}
	settings := doctorSettingsPath(t)
	c := secretPresenceCheck(getenv, func() string { return settings })
	ok, detail := c.Run()
	if strings.Contains(detail, "secretpass") || strings.Contains(detail, "10.0.0.1") || strings.Contains(detail, "mongodb://") {
		t.Fatalf("FR-041 VIOLATION: secret value leaked into detail: %q", detail)
	}
	if !strings.Contains(detail, "MONGO_URL") {
		t.Fatalf("detail should name the key MONGO_URL: %q", detail)
	}
	_ = ok
}

func TestSecretPresenceReportsAbsent(t *testing.T) {
	settings := doctorSettingsPath(t)
	c := secretPresenceCheck(func(string) string { return "" }, func() string { return settings })
	ok, detail := c.Run()
	if ok {
		t.Fatal("no MONGO_URL anywhere → check should be not-ok")
	}
	if !strings.Contains(detail, "MONGO_URL") {
		t.Fatalf("should name the missing key: %q", detail)
	}
}

func TestSecretPresenceCheck_EnvMongoUrlCountsForProfile(t *testing.T) {
	getenv := func(k string) string {
		if k == "MONGO_URL" {
			return "mongodb://x"
		}
		return ""
	}
	settings := doctorSettingsPath(t)
	c := secretPresenceCheck(getenv, func() string { return settings })
	ok, detail := c.Run()
	if !ok {
		t.Fatalf("MONGO_URL is set in env, ok should be true; detail=%q", detail)
	}
	if !strings.Contains(detail, "profile=staging") {
		t.Fatalf("detail should identify the checked profile: %q", detail)
	}
}

// F3: with neither env nor a workspace, the check stays not-ok and now
// explains why via the no-workspace hint (previously an early return with a
// fixed message that ignored env entirely).
func TestSecretPresenceCheck_NoEnvNoWorkspace(t *testing.T) {
	c := secretPresenceCheck(func(string) string { return "" }, func() string { return "" })
	ok, detail := c.Run()
	if ok {
		t.Fatal("no MONGO_URL and no workspace → check should be not-ok")
	}
	if !strings.Contains(detail, "no workspace") {
		t.Fatalf("detail should explain no workspace: %q", detail)
	}
}

func TestToolPresenceReportsMissing(t *testing.T) {
	c := toolPresenceCheck([]string{"this-binary-does-not-exist-zzz"})
	ok, detail := c.Run()
	if ok {
		t.Fatal("a nonexistent tool should make the check not-ok")
	}
	if !strings.Contains(detail, "this-binary-does-not-exist-zzz") {
		t.Fatalf("should name the missing tool: %q", detail)
	}
}

func TestPlaywrightCheckIsReadOnlyRegistered(t *testing.T) {
	c := playwrightCheck()
	if c.Name != "playwright" {
		t.Fatalf("name: %q", c.Name)
	}
	// Run must not panic and must return a detail mentioning mcp state.
	_, detail := c.Run()
	if !strings.Contains(detail, "mcp=") {
		t.Fatalf("detail should report mcp state: %q", detail)
	}
}

func TestPluginCheckReportsMissing(t *testing.T) {
	c := pluginCheckAt(t.TempDir()) // empty dest → not synced
	ok, msg := c.Run()
	if ok {
		t.Fatal("plugin not synced but reports ok")
	}
	if !strings.Contains(msg, "chưa cài") { //znf:allow-lang
		t.Fatalf("msg should state the status: %q", msg)
	}
}

func TestDockerCheck_PresentAndRunning(t *testing.T) {
	c := dockerCheckWith(
		func(string) (string, error) { return "/usr/bin/docker", nil },
		func() error { return nil },
	)
	ok, detail := c.Run()
	if !ok {
		t.Errorf("docker present + info ok should be OK, detail=%q", detail)
	}
}

func TestDockerCheck_Missing(t *testing.T) {
	c := dockerCheckWith(
		func(string) (string, error) { return "", errors.New("not found") },
		func() error { return nil },
	)
	ok, detail := c.Run()
	if ok || !strings.Contains(detail, "docker=missing") {
		t.Errorf("docker missing should be !ok + detail containing docker=missing, got ok=%v detail=%q", ok, detail)
	}
}

// The git-guard check must fail on a fresh HOME, its Fix must wire the hook,
// and the check must then pass — the path `doctor --fix` takes.
func TestGitGuardCheck_MissingThenFixed(t *testing.T) {
	home := t.TempDir()
	c := gitGuardCheck(func() (string, error) { return home, nil })
	if ok, detail := c.Run(); ok || !strings.Contains(detail, "hook=missing") {
		t.Fatalf("fresh HOME should report hook=missing, got ok=%v detail=%q", ok, detail)
	}
	if ok, detail := c.Fix(); !ok {
		t.Fatalf("Fix failed: %s", detail)
	}
	if ok, detail := c.Run(); !ok {
		t.Fatalf("after Fix the check should pass, detail=%q", detail)
	}
}

func TestGitIdentityCheck_Unknown(t *testing.T) {
	c := gitIdentityCheckWith(func(v string) bool { return v == "GIT_AUTHOR_IDENT" })
	ok, detail := c.Run()
	if ok || !strings.Contains(detail, "GIT_COMMITTER_IDENT=unknown") || strings.Contains(detail, "GIT_AUTHOR_IDENT") {
		t.Errorf("want !ok naming only the committer, got ok=%v detail=%q", ok, detail)
	}
}

func TestGitIdentityCheck_Present(t *testing.T) {
	c := gitIdentityCheckWith(func(string) bool { return true })
	if ok, detail := c.Run(); !ok {
		t.Errorf("name+email set should be OK, detail=%q", detail)
	}
}

func TestKitPermissionsCheck_MissingThenFixed(t *testing.T) {
	home := t.TempDir()
	c := kitPermissionsCheck(func() (string, error) { return home, nil })
	if ok, detail := c.Run(); ok || !strings.Contains(detail, "missing") {
		t.Fatalf("empty HOME should report missing, got ok=%v detail=%q", ok, detail)
	}
	if ok, detail := c.Fix(); !ok {
		t.Fatalf("Fix failed: %s", detail)
	}
	if ok, detail := c.Run(); !ok {
		t.Fatalf("after Fix the check should pass, detail=%q", detail)
	}
}
