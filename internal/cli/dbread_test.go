package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ZenifyAIContactCenter/zenify-kit/internal/dbread"
)

func TestRedactEnvValues(t *testing.T) {
	secret := "mongodb://user:password@example.invalid"
	got := string(redactEnvValues([]byte("connection failed: "+secret), []string{"ZENIFY_DB_READ_MONGO_URI=" + secret}))
	if strings.Contains(got, secret) || got != "connection failed: [REDACTED]" {
		t.Fatalf("redacted output = %q", got)
	}
}

func TestMergeEnvReplacesCredentialInsteadOfDuplicatingIt(t *testing.T) {
	got := mergeEnv([]string{"PATH=/bin", "MONGO_URL=old", "MYSQL_PWD=old"}, []string{"MONGO_URL=", "MYSQL_PWD=new"})
	want := []string{"PATH=/bin", "MONGO_URL=", "MYSQL_PWD=new"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("mergeEnv = %v, want %v", got, want)
	}
}

func TestDBReadProfileDefaultsToStaging(t *testing.T) {
	flag := newDBReadCmd().Flags().Lookup("profile")
	if flag == nil || flag.DefValue != "staging" {
		t.Fatalf("profile flag = %#v, want default staging", flag)
	}
}

func TestDefaultProfileEnv(t *testing.T) {
	cases := map[string]string{
		"staging":    "MONGO_URL",
		"local":      "MONGO_URL_LOCAL",
		"develop":    "MONGO_URL_DEVELOP",
		"custom":     "MONGO_URL_CUSTOM",
		"production": "MONGO_URL_PRODUCTION",
	}
	for profile, want := range cases {
		if got := defaultProfileEnv("MONGO_URL", profile); got != want {
			t.Errorf("defaultProfileEnv(MONGO_URL, %s) = %q, want %q", profile, got, want)
		}
	}
}

func TestRequiredEnvKey(t *testing.T) {
	for _, valid := range []string{"MONGO_URL", "mysql_host_2", "_PRIVATE"} {
		if err := requiredEnvKey(valid); err != nil {
			t.Errorf("requiredEnvKey(%q): %v", valid, err)
		}
	}
	for _, invalid := range []string{"", "2_MONGO", "MONGO-URL", "MONGO URL"} {
		if err := requiredEnvKey(invalid); err == nil {
			t.Errorf("requiredEnvKey(%q) unexpectedly passed", invalid)
		}
	}
}

func TestDBReadSetupUsesRequestedProfileAndDoesNotPrintCredential(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".zenify"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".zenify", "manifest.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)

	secret := "mongodb://local-user:local-password@example.invalid"
	var initial string
	var choose bool
	cmd := newDBReadCmdWithSetup(func(initialProfile string, chooseProfile bool) (dbread.SetupInput, error) {
		initial, choose = initialProfile, chooseProfile
		return dbread.SetupInput{
			Name:        "local",
			Profile:     dbread.Profile{Mongo: &dbread.MongoProfile{URIEnv: "MONGO_URL_LOCAL", Database: "app_local"}},
			Credentials: map[string]string{"MONGO_URL_LOCAL": secret},
		}, nil
	})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"--profile", "local", "setup"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if initial != "local" || choose {
		t.Fatalf("prompt args = (%q, %v), want (local, false)", initial, choose)
	}
	if strings.Contains(stdout.String(), secret) {
		t.Fatal("setup printed a credential")
	}
	if _, err := os.Stat(filepath.Join(root, ".zenify", "db-read.json")); err != nil {
		t.Fatalf("profile metadata was not written: %v", err)
	}
	settings, err := os.ReadFile(filepath.Join(root, ".claude", "settings.local.json"))
	if err != nil {
		t.Fatalf("settings were not written: %v", err)
	}
	if !bytes.Contains(settings, []byte(secret)) {
		t.Fatal("credential was not persisted in settings.local.json")
	}
}
