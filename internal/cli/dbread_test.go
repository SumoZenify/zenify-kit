package cli

import (
	"strings"
	"testing"
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
