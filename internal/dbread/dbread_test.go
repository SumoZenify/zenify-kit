package dbread

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRefuseWrites(t *testing.T) {
	cases := []struct {
		stmt string
		want bool
	}{
		{"INSERT", true},
		{"Db.X.updateOne", true},
		{"SET x", true},
		{"SET a=1", true},
		{"db.coll.find({})", false},
		{"SELECT * FROM t", false},
		{"db.coll.aggregate([...])", false},
		{"estimatedDocumentCount()", false},
		{"reset()", false},
		{"db.coll.deleteOne({})", true},
		{"db.coll.remove({})", true},
		{"db.coll.replaceOne({},{})", true},
		{"db.coll.drop()", true},
		{"db.coll.renameCollection('x')", true},
		{"db.coll.save({})", true},
		{"db.coll.bulkWrite([])", true},
		{"db.coll.findAndModify({})", true},
		{"db.runCommand({collMod: 'x'})", true},
		{"TRUNCATE TABLE t", true},
		{"ALTER TABLE t", true},
		{"GRANT ALL ON x TO y", true},
		{"REVOKE ALL ON x FROM y", true},
		{"CREATE TABLE t (id int)", true},
		{`db.events.aggregate([{$out:"archive"}])`, true},
		{`db.events.aggregate([{$merge:"archive"}])`, true},
		{"CALL refresh_cache()", true},
		{"CALL(refresh_cache())", true},
		{"SELECT * FROM t INTO OUTFILE '/tmp/x'", true},
		{"SELECT callback AS call FROM events", false},
		{"SELECT 1; CALL refresh_cache()", true},
	}
	for _, c := range cases {
		if got := RefuseWrites(c.stmt); got != c.want {
			t.Errorf("RefuseWrites(%q) = %v, want %v", c.stmt, got, c.want)
		}
	}
}

// baseOptions returns Options wired with a fake settings path (empty dir, so
// LoadCreds falls through to env), fresh buffers, and env carrying a fake
// MONGO_URL/MYSQL_* set so Run doesn't hit the "not set" fatal path.
func baseOptions(t *testing.T, env map[string]string) (*Options, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, ".claude", "settings.local.json")
	configPath := filepath.Join(dir, ".zenify", "db-read.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	config := ProfileConfig{Profiles: map[string]Profile{
		"staging": {
			Mongo: &MongoProfile{URIEnv: "MONGO_URL", Database: "app_staging"},
			MySQL: &MySQLProfile{HostEnv: "MYSQL_HOST", PortEnv: "MYSQL_PORT", UserEnv: "MYSQL_USER", PasswordEnv: "MYSQL_PASSWORD", Database: "acd_staging"},
		},
		"develop": {Mongo: &MongoProfile{URIEnv: "MONGO_URL_DEVELOP", Database: "app_develop"}},
	}}
	b, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, b, 0o600); err != nil {
		t.Fatal(err)
	}
	o := &Options{
		Stdout:       &stdout,
		Stderr:       &stderr,
		Env:          func(k string) string { return env[k] },
		SettingsPath: settingsPath,
		ConfigPath:   configPath,
	}
	return o, &stdout, &stderr
}

const fakeMongoURL = "mongodb://u:p@1.2.3.4:27017/?authSource=app_staging" //nolint:gosec // G101 -- test fixture literal, not a real credential
const fakeMysqlPassword = "s3cr3t-pw"

func fakeEnv() map[string]string {
	return map[string]string{
		"MONGO_URL":      fakeMongoURL,
		"MYSQL_HOST":     "mysql.internal",
		"MYSQL_PORT":     "3306",
		"MYSQL_USER":     "clusteradmin",
		"MYSQL_PASSWORD": fakeMysqlPassword,
	}
}

func hasAssignment(env []string, want string) bool {
	for _, value := range env {
		if value == want {
			return true
		}
	}
	return false
}

func TestRun_Collections_ExactEvalAndArgs(t *testing.T) {
	o, stdout, _ := baseOptions(t, fakeEnv())
	var capturedName string
	var capturedArgs, capturedEnv []string
	o.SetRun(func(name string, args []string, extraEnv []string, stdin string) error {
		capturedName = name
		capturedArgs = args
		capturedEnv = extraEnv
		_, _ = fmt.Fprint(o.Stdout, "chat_rooms\nusers\ntickets\n")
		return nil
	})
	o.Cmd = "collections"
	if err := Run(o); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if capturedName != "mongosh" {
		t.Errorf("name = %q, want mongosh", capturedName)
	}
	wantArgs := []string{"--nodb", "--norc", "--quiet", "--eval",
		`db = (() => { const uri = process.env.ZENIFY_DB_READ_MONGO_URI; delete process.env.ZENIFY_DB_READ_MONGO_URI; return new Mongo(uri).getDB("app_staging"); })(); db.getCollectionInfos({type:"collection"}).map(c=>c.name).sort().forEach(n=>print(n))`}
	if len(capturedArgs) != len(wantArgs) {
		t.Fatalf("args = %v, want %v", capturedArgs, wantArgs)
	}
	if !hasAssignment(capturedEnv, "ZENIFY_DB_READ_MONGO_URI="+fakeMongoURL) || !hasAssignment(capturedEnv, "MONGO_URL=") {
		t.Errorf("extraEnv = %v, want Mongo URI only in child env", capturedEnv)
	}
	for _, arg := range capturedArgs {
		if strings.Contains(arg, fakeMongoURL) {
			t.Fatal("Mongo URI leaked into process arguments")
		}
	}
	for i := range wantArgs {
		if capturedArgs[i] != wantArgs[i] {
			t.Errorf("args[%d] = %q, want %q", i, capturedArgs[i], wantArgs[i])
		}
	}
	if stdout.String() != "chat_rooms\nusers\ntickets\n" {
		t.Errorf("stdout = %q", stdout.String())
	}
}

func TestRun_Collections_ArgFilters(t *testing.T) {
	o, stdout, _ := baseOptions(t, fakeEnv())
	o.SetRun(func(name string, args []string, extraEnv []string, stdin string) error {
		_, _ = fmt.Fprint(o.Stdout, "chat_rooms\nusers\ntickets\nchat_messages\n")
		return nil
	})
	o.Cmd = "collections"
	o.Arg = "chat"
	if err := Run(o); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if stdout.String() != "chat_rooms\nchat_messages" {
		t.Errorf("stdout = %q", stdout.String())
	}
}

func TestRun_DerivesProfileConfigFromWorkspaceSettings(t *testing.T) {
	o, _, _ := baseOptions(t, fakeEnv())
	o.ConfigPath = ""
	o.Cmd = "collections"
	o.SetRun(func(string, []string, []string, string) error { return nil })
	if err := Run(o); err != nil {
		t.Fatalf("Run: %v", err)
	}
}

func TestRun_ProfileSelectsMatchingDatabaseAndCredential(t *testing.T) {
	env := fakeEnv()
	env["MONGO_URL_DEVELOP"] = "mongodb://develop.example.invalid"
	o, _, stderr := baseOptions(t, env)
	o.Cmd = "collections"
	o.Profile = "develop"
	var capturedArgs, capturedEnv []string
	o.SetRun(func(_ string, args []string, extraEnv []string, _ string) error {
		capturedArgs, capturedEnv = args, extraEnv
		return nil
	})
	if err := Run(o); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(capturedArgs[4], `.getDB("app_develop")`) {
		t.Errorf("eval = %q, want develop database", capturedArgs[4])
	}
	if !hasAssignment(capturedEnv, "ZENIFY_DB_READ_MONGO_URI="+env["MONGO_URL_DEVELOP"]) || !hasAssignment(capturedEnv, "MONGO_URL=") {
		t.Errorf("extraEnv = %v, want develop credential", capturedEnv)
	}
	if !strings.Contains(stderr.String(), "profile=develop") {
		t.Errorf("stderr = %q, want selected profile", stderr.String())
	}
}

func TestRun_MissingProfileDoesNotFallBack(t *testing.T) {
	o, _, _ := baseOptions(t, fakeEnv())
	o.Cmd = "collections"
	o.Profile = "production"
	called := false
	o.SetRun(func(string, []string, []string, string) error { called = true; return nil })
	err := Run(o)
	if err == nil || !strings.Contains(err.Error(), `profile "production" is not configured`) {
		t.Fatalf("err = %v, want missing profile error", err)
	}
	if called {
		t.Fatal("missing profile fell back and executed a query")
	}
}

func TestSaveProfileSeparatesMetadataAndCredentials(t *testing.T) {
	root := t.TempDir()
	settingsPath := filepath.Join(root, ".claude", "settings.local.json")
	secretURI := "mongodb://user:password@example.invalid/db?retryWrites=true&w=majority"
	secretPassword := "mysql-test-password"
	input := SetupInput{
		Name: "develop",
		Profile: Profile{
			Mongo: &MongoProfile{URIEnv: "MONGO_URL_DEVELOP", Database: "app_develop"},
			MySQL: &MySQLProfile{
				HostEnv: "MYSQL_HOST_DEVELOP", PortEnv: "MYSQL_PORT_DEVELOP",
				UserEnv: "MYSQL_USER_DEVELOP", PasswordEnv: "MYSQL_PASSWORD_DEVELOP",
				Database: "acd_develop",
			},
		},
		Credentials: map[string]string{
			"MONGO_URL_DEVELOP":      secretURI,
			"MYSQL_HOST_DEVELOP":     "mysql.example.invalid",
			"MYSQL_PORT_DEVELOP":     "3307",
			"MYSQL_USER_DEVELOP":     "readonly",
			"MYSQL_PASSWORD_DEVELOP": secretPassword,
			"UNRELATED_SECRET":       "must-not-be-written",
		},
	}

	configPath, err := SaveProfile(settingsPath, input)
	if err != nil {
		t.Fatalf("SaveProfile: %v", err)
	}
	configData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(configData), secretURI) || strings.Contains(string(configData), secretPassword) {
		t.Fatal("profile metadata contains a credential value")
	}
	var cfg ProfileConfig
	if err := json.Unmarshal(configData, &cfg); err != nil {
		t.Fatal(err)
	}
	if got := cfg.Profiles["develop"].Mongo.Database; got != "app_develop" {
		t.Fatalf("Mongo database = %q", got)
	}

	settingsData, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	var settings struct {
		Env map[string]string `json:"env"`
	}
	if err := json.Unmarshal(settingsData, &settings); err != nil {
		t.Fatal(err)
	}
	if settings.Env["MONGO_URL_DEVELOP"] != secretURI || settings.Env["MYSQL_PASSWORD_DEVELOP"] != secretPassword {
		t.Fatal("settings did not contain the entered credentials")
	}
	if _, ok := settings.Env["UNRELATED_SECRET"]; ok {
		t.Fatal("SaveProfile wrote a credential key not declared by the profile")
	}
	info, err := os.Stat(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("settings mode = %o, want 600", info.Mode().Perm())
	}
}

func TestSaveProfilePreservesExistingProfilesSettingsAndCredentials(t *testing.T) {
	root := t.TempDir()
	settingsPath := filepath.Join(root, ".claude", "settings.local.json")
	configPath := filepath.Join(root, ".zenify", "db-read.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(configPath), 0o750); err != nil {
		t.Fatal(err)
	}
	existingSecret := "mongodb://existing.invalid/?x=1&y=2"
	settingsData, _ := json.Marshal(map[string]any{
		"permissions": map[string]any{"allow": []string{"Bash(git status)"}},
		"env":         map[string]any{"MONGO_URL_LOCAL": existingSecret, "OTHER": "keep"},
	})
	if err := os.WriteFile(settingsPath, settingsData, 0o644); err != nil {
		t.Fatal(err)
	}
	existingConfig := ProfileConfig{Profiles: map[string]Profile{
		"staging": {Mongo: &MongoProfile{URIEnv: "MONGO_URL", Database: "app_staging"}},
	}}
	configData, _ := json.Marshal(existingConfig)
	if err := os.WriteFile(configPath, configData, 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := SaveProfile(settingsPath, SetupInput{
		Name:        "local",
		Profile:     Profile{Mongo: &MongoProfile{URIEnv: "MONGO_URL_LOCAL", Database: "app_local"}},
		Credentials: map[string]string{"MONGO_URL_LOCAL": ""},
	})
	if err != nil {
		t.Fatalf("SaveProfile: %v", err)
	}

	var gotCfg ProfileConfig
	data, _ := os.ReadFile(configPath)
	if err := json.Unmarshal(data, &gotCfg); err != nil {
		t.Fatal(err)
	}
	if gotCfg.Profiles["staging"].Mongo == nil || gotCfg.Profiles["local"].Mongo == nil {
		t.Fatalf("profiles = %#v, want staging and local", gotCfg.Profiles)
	}
	var gotSettings map[string]any
	data, _ = os.ReadFile(settingsPath)
	if err := json.Unmarshal(data, &gotSettings); err != nil {
		t.Fatal(err)
	}
	env := gotSettings["env"].(map[string]any)
	if env["MONGO_URL_LOCAL"] != existingSecret || env["OTHER"] != "keep" || gotSettings["permissions"] == nil {
		t.Fatalf("settings were not preserved: %#v", gotSettings)
	}
}

func TestSaveProfileMissingCredentialDoesNotWrite(t *testing.T) {
	root := t.TempDir()
	settingsPath := filepath.Join(root, ".claude", "settings.local.json")
	_, err := SaveProfile(settingsPath, SetupInput{
		Name:    "production",
		Profile: Profile{Mongo: &MongoProfile{URIEnv: "MONGO_URL_PRODUCTION", Database: "app_production"}},
	})
	if err == nil || !strings.Contains(err.Error(), "MONGO_URL_PRODUCTION") {
		t.Fatalf("err = %v, want missing env key", err)
	}
	if _, statErr := os.Stat(settingsPath); !os.IsNotExist(statErr) {
		t.Fatalf("settings file was written after rejected setup: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(root, ".zenify", "db-read.json")); !os.IsNotExist(statErr) {
		t.Fatalf("config file was written after rejected setup: %v", statErr)
	}
}

func TestSaveProfileAllowsMySQLDefaultPort(t *testing.T) {
	root := t.TempDir()
	settingsPath := filepath.Join(root, ".claude", "settings.local.json")
	_, err := SaveProfile(settingsPath, SetupInput{
		Name: "custom",
		Profile: Profile{MySQL: &MySQLProfile{
			HostEnv: "MYSQL_HOST_CUSTOM", PortEnv: "MYSQL_PORT_CUSTOM",
			UserEnv: "MYSQL_USER_CUSTOM", PasswordEnv: "MYSQL_PASSWORD_CUSTOM",
			Database: "app_custom",
		}},
		Credentials: map[string]string{
			"MYSQL_HOST_CUSTOM":     "mysql.example.invalid",
			"MYSQL_USER_CUSTOM":     "readonly",
			"MYSQL_PASSWORD_CUSTOM": "fake-password",
		},
	})
	if err != nil {
		t.Fatalf("SaveProfile with default MySQL port: %v", err)
	}
}

func TestRun_DocEscapesCollectionName(t *testing.T) {
	o, _, _ := baseOptions(t, fakeEnv())
	o.Cmd = "doc"
	o.Arg = `events'); db.events.drop(); ('`
	var eval string
	o.SetRun(func(_ string, args []string, _ []string, _ string) error { eval = args[4]; return nil })
	if err := Run(o); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if strings.Contains(eval, "getCollection('events')") || !strings.Contains(eval, `getCollection("events'); db.events.drop(); ('")`) {
		t.Errorf("collection argument was not JSON encoded: %q", eval)
	}
}

func TestRun_Eval_WriteRefused(t *testing.T) {
	o, _, stderr := baseOptions(t, fakeEnv())
	called := false
	o.SetRun(func(name string, args []string, extraEnv []string, stdin string) error {
		called = true
		return nil
	})
	o.Cmd = "eval"
	o.Arg = "db.tickets.insertOne({})"
	if err := Run(o); err == nil {
		t.Fatal("Run: want error for a write expression")
	}
	if called {
		t.Error("run seam was invoked for a refused write")
	}
	if !strings.Contains(stderr.String(), "refusing") {
		t.Errorf("stderr = %q, want refusal message", stderr.String())
	}
}

func TestRun_Eval_ReadPassesThrough(t *testing.T) {
	o, _, _ := baseOptions(t, fakeEnv())
	var capturedArgs []string
	o.SetRun(func(name string, args []string, extraEnv []string, stdin string) error {
		capturedArgs = args
		return nil
	})
	o.Cmd = "eval"
	o.Arg = "db.tickets.find({}).limit(1)"
	if err := Run(o); err != nil {
		t.Fatalf("Run: %v", err)
	}
	wantJS := `db = (() => { const uri = process.env.ZENIFY_DB_READ_MONGO_URI; delete process.env.ZENIFY_DB_READ_MONGO_URI; return new Mongo(uri).getDB("app_staging"); })(); db.tickets.find({}).limit(1)`
	if len(capturedArgs) < 5 || capturedArgs[4] != wantJS {
		t.Errorf("args = %v, want eval js %q", capturedArgs, wantJS)
	}
}

func TestRun_SQL_WriteRefused(t *testing.T) {
	o, _, _ := baseOptions(t, fakeEnv())
	called := false
	o.SetRun(func(name string, args []string, extraEnv []string, stdin string) error {
		called = true
		return nil
	})
	o.Cmd = "sql"
	o.Arg = "DELETE FROM agents"
	if err := Run(o); err == nil {
		t.Fatal("Run: want error for a write query")
	}
	if called {
		t.Error("run seam was invoked for a refused write")
	}
}

func TestRun_SQL_ReadPassesThroughWithPassword(t *testing.T) {
	o, stdout, stderr := baseOptions(t, fakeEnv())
	var capturedName string
	var capturedArgs, capturedExtraEnv []string
	o.SetRun(func(name string, args []string, extraEnv []string, stdin string) error {
		capturedName = name
		capturedArgs = args
		capturedExtraEnv = extraEnv
		_, _ = fmt.Fprint(o.Stdout, "agents\n")
		return nil
	})
	o.Cmd = "sql"
	o.Arg = "SELECT * FROM agents"
	if err := Run(o); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if capturedName != "mysql" {
		t.Errorf("name = %q, want mysql", capturedName)
	}
	found := false
	for _, e := range capturedExtraEnv {
		if e == "MYSQL_PWD="+fakeMysqlPassword {
			found = true
		}
	}
	if !found {
		t.Errorf("extraEnv = %v, want MYSQL_PWD=%s", capturedExtraEnv, fakeMysqlPassword)
	}
	wantArgs := []string{"--no-defaults", "--protocol=TCP", "-h", "mysql.internal", "-P", "3306", "-u", "clusteradmin", "-N", "-B", "-e", "SELECT * FROM agents", "acd_staging"}
	if len(capturedArgs) != len(wantArgs) {
		t.Fatalf("args = %v, want %v", capturedArgs, wantArgs)
	}
	for i := range wantArgs {
		if capturedArgs[i] != wantArgs[i] {
			t.Errorf("args[%d] = %q, want %q", i, capturedArgs[i], wantArgs[i])
		}
	}
	if strings.Contains(stdout.String(), fakeMysqlPassword) || strings.Contains(stderr.String(), fakeMysqlPassword) {
		t.Error("FR-041 violation: password leaked into stdout/stderr")
	}
}

func TestRun_SQLDoesNotRequireMongoCredential(t *testing.T) {
	env := fakeEnv()
	delete(env, "MONGO_URL")
	o, _, _ := baseOptions(t, env)
	o.Cmd = "sql"
	o.Arg = "SELECT 1"
	o.SetRun(func(string, []string, []string, string) error { return nil })
	if err := Run(o); err != nil {
		t.Fatalf("Run: %v", err)
	}
}

func TestRun_ProfileCredentialsFallBackToWorkspaceSettings(t *testing.T) {
	o, _, _ := baseOptions(t, map[string]string{})
	if err := os.MkdirAll(filepath.Dir(o.SettingsPath), 0o755); err != nil {
		t.Fatal(err)
	}
	settings := map[string]any{"env": map[string]string{"MONGO_URL": fakeMongoURL}}
	b, err := json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(o.SettingsPath, b, 0o600); err != nil {
		t.Fatal(err)
	}
	o.Cmd = "collections"
	var childEnv []string
	o.SetRun(func(_ string, _ []string, extraEnv []string, _ string) error { childEnv = extraEnv; return nil })
	if err := Run(o); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !hasAssignment(childEnv, "ZENIFY_DB_READ_MONGO_URI="+fakeMongoURL) || !hasAssignment(childEnv, "MONGO_URL=") {
		t.Errorf("extraEnv = %v, want credential from workspace settings", childEnv)
	}
}

func TestRun_NoMongoURL_FatalNoRun(t *testing.T) {
	o, _, _ := baseOptions(t, map[string]string{})
	called := false
	o.SetRun(func(name string, args []string, extraEnv []string, stdin string) error {
		called = true
		return nil
	})
	o.Cmd = "collections"
	err := Run(o)
	if err == nil {
		t.Fatal("Run: want error when MONGO_URL is unset")
	}
	if called {
		t.Error("run seam was invoked despite missing MONGO_URL")
	}
	if !strings.Contains(err.Error(), "MONGO_URL is not set") {
		t.Errorf("err = %q, want MONGO_URL message", err)
	}
}

func TestRun_ArgRequired(t *testing.T) {
	for _, cmd := range []string{"doc", "count", "eval", "sql"} {
		o, _, _ := baseOptions(t, fakeEnv())
		called := false
		o.SetRun(func(name string, args []string, extraEnv []string, stdin string) error {
			called = true
			return nil
		})
		o.Cmd = cmd
		o.Arg = ""
		if err := Run(o); err == nil {
			t.Errorf("Run(%s): want error for empty arg", cmd)
		}
		if called {
			t.Errorf("Run(%s): run seam invoked despite missing arg", cmd)
		}
	}
}

func TestRun_FR041_NoSecretLeak(t *testing.T) {
	// Exercises collections, eval-read, and sql-read together and asserts
	// the fake MONGO_URL and MySQL password never land in captured output.
	scenarios := []func(*Options){
		func(o *Options) { o.Cmd = "collections" },
		func(o *Options) { o.Cmd = "eval"; o.Arg = "db.tickets.find({})" },
		func(o *Options) { o.Cmd = "sql"; o.Arg = "SELECT 1" },
	}
	for _, setup := range scenarios {
		o, stdout, stderr := baseOptions(t, fakeEnv())
		o.SetRun(func(name string, args []string, extraEnv []string, stdin string) error {
			_, _ = fmt.Fprint(o.Stdout, "some-output\n")
			return nil
		})
		setup(o)
		if err := Run(o); err != nil {
			t.Fatalf("Run(%s): %v", o.Cmd, err)
		}
		combined := stdout.String() + stderr.String()
		if strings.Contains(combined, fakeMongoURL) {
			t.Errorf("Run(%s): MONGO_URL leaked into output: %q", o.Cmd, combined)
		}
		if strings.Contains(combined, fakeMysqlPassword) {
			t.Errorf("Run(%s): MySQL password leaked into output: %q", o.Cmd, combined)
		}
	}
}

func TestFilterLines(t *testing.T) {
	out := "chat_rooms\nusers\nchat_messages\nTICKETS\n"
	if got := filterLines(out, "chat"); got != "chat_rooms\nchat_messages" {
		t.Errorf("filterLines chat = %q", got)
	}
	if got := filterLines(out, "TICK"); got != "TICKETS" {
		t.Errorf("filterLines TICK (case-insensitive) = %q", got)
	}
	if got := filterLines(out, ""); got != out {
		t.Errorf("filterLines empty substr = %q, want unchanged", got)
	}
}

func TestRun_NoWorkspaceNoEnv_FriendlyError(t *testing.T) {
	var out, errb bytes.Buffer
	o := &Options{Cmd: "collections", Stdout: &out, Stderr: &errb, Env: func(string) string { return "" }}
	o.SetRun(func(string, []string, []string, string) error { t.Fatal("must not run mongosh"); return nil })
	err := Run(o)
	if err == nil || !strings.Contains(err.Error(), "no workspace") {
		t.Fatalf("err=%v stderr=%q", err, errb.String())
	}
}
