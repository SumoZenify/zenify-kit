// Package dbread ports the workspace's `db_read` POSIX shell tool
// (~/.local/bin/db_read) to a portable Go implementation (FR-033).
//
// It shells out to `mongosh`/`mysql` rather than using a native Go DB
// driver: `eval` runs arbitrary mongosh JavaScript, which a native driver
// cannot evaluate, and shelling to the same CLIs the bash tool used
// guarantees byte-identical output for every other subcommand too.
//
// FR-041: credentials are read into memory and passed to child processes only
// through their environment. No code path in this package prints, logs, or
// echoes a connection string or password.
package dbread

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

// Options configures one Run. Stdout/Stderr and the run/Env seams let tests
// inject without spawning real processes or touching a real settings file.
//
// Options is always passed to Run by pointer: for the collections/tables
// substring filter, Run needs to redirect where the run seam writes (to an
// in-memory buffer, before filtering the result into the real Stdout), and
// the injected run closure reads o.Stdout dynamically at call time — that
// only works if Run and the closure share the same Options.
type Options struct {
	Cmd, Arg     string
	Profile      string
	Stdout       io.Writer
	Stderr       io.Writer
	Env          func(string) string
	SettingsPath string
	ConfigPath   string
	run          func(name string, args []string, extraEnv []string, stdin string) error
}

// SetRun injects the process-spawning seam. Exported so the cobra command
// (internal/cli) can wire the real os/exec runner without this package
// exposing the run field directly; unit tests in this package set it too.
func (o *Options) SetRun(run func(name string, args []string, extraEnv []string, stdin string) error) {
	o.run = run
}

// RefuseWrites reports true if stmt looks like a mutation and must be
// rejected. Advisory guard (a determined expression can evade a denylist)
// — the real boundary is a database account with read-only permissions.
// Match case-insensitively against mutation verbs as an accident guard.
var directCallPattern = regexp.MustCompile(`(?i)(^|;)\s*call\b`)

func RefuseWrites(stmt string) bool {
	s := strings.ToLower(stmt)
	needles := []string{
		"insert", "update", "delete", "remove", "replace", "drop", "create",
		"rename", "save(", "bulkwrite", "findandmodify", "collmod", "truncate",
		"alter", "grant", "revoke", "set ",
		"$out", "$merge", "into outfile", "load data",
	}
	for _, n := range needles {
		if strings.Contains(s, n) {
			return true
		}
	}
	return directCallPattern.MatchString(stmt)
}

// filterLines keeps only the lines of out containing substr, case-insensitively.
// An empty substr returns out unchanged.
func filterLines(out, substr string) string {
	if substr == "" {
		return out
	}
	var keep []string
	low := strings.ToLower(substr)
	for _, l := range strings.Split(out, "\n") {
		if strings.Contains(strings.ToLower(l), low) {
			keep = append(keep, l)
		}
	}
	return strings.Join(keep, "\n")
}

// Run dispatches one db-read subcommand through a workspace profile.
func Run(o *Options) error {
	if o.Env == nil {
		o.Env = os.Getenv
	}
	if o.Profile == "" {
		o.Profile = "staging"
	}
	if o.ConfigPath == "" {
		o.ConfigPath = profileConfigPath(o.SettingsPath)
	}
	profile, err := loadProfile(o.ConfigPath, o.Profile)
	if err != nil {
		return err
	}
	settings := loadSettingsEnv(o.SettingsPath)
	get := func(key string) string { return resolveEnv(o.Env, settings, key) }
	credentialClears := credentialEnvClears(o.ConfigPath)
	jsString := func(value string) string {
		b, _ := json.Marshal(value)
		return string(b)
	}
	printTarget := func(engine, database string) {
		_, _ = fmt.Fprintf(o.Stderr, "zenify db-read: profile=%s engine=%s database=%s\n", o.Profile, engine, database)
	}

	mongoEval := func(js string) error {
		if profile.Mongo == nil {
			return fmt.Errorf("profile %q has no Mongo configuration", o.Profile)
		}
		uri := get(profile.Mongo.URIEnv)
		if uri == "" {
			return fmt.Errorf("profile %q: %s is not set", o.Profile, profile.Mongo.URIEnv)
		}
		printTarget("mongo", profile.Mongo.Database)
		prefix := "db = (() => { const uri = process.env.ZENIFY_DB_READ_MONGO_URI; delete process.env.ZENIFY_DB_READ_MONGO_URI; return new Mongo(uri).getDB(" + jsString(profile.Mongo.Database) + "); })(); "
		childEnv := append(append([]string(nil), credentialClears...), "ZENIFY_DB_READ_MONGO_URI="+uri)
		err := o.run("mongosh", []string{"--nodb", "--norc", "--quiet", "--eval", prefix + js}, childEnv, "")
		if err != nil {
			_, _ = fmt.Fprintln(o.Stderr, "zenify db-read: mongosh failed; check profile credentials and network access")
		}
		return err
	}

	mysqlRun := func(sql string) error {
		if profile.MySQL == nil {
			return fmt.Errorf("profile %q has no MySQL configuration", o.Profile)
		}
		mysql := profile.MySQL
		host, user, password := get(mysql.HostEnv), get(mysql.UserEnv), get(mysql.PasswordEnv)
		if host == "" || user == "" || password == "" {
			return fmt.Errorf("profile %q: MySQL host, user or password is not set", o.Profile)
		}
		port := get(mysql.PortEnv)
		if port == "" {
			port = "3306"
		}
		printTarget("mysql", mysql.Database)
		extraEnv := append(append([]string(nil), credentialClears...), "MYSQL_PWD="+password)
		return o.run("mysql", []string{
			"--no-defaults", "--protocol=TCP", "-h", host, "-P", port, "-u", user,
			"-N", "-B", "-e", sql, mysql.Database,
		}, extraEnv, "")
	}

	// runFiltered redirects o.Stdout to a buffer for the duration of fn,
	// then writes the substring-filtered result to the real o.Stdout.
	runFiltered := func(fn func() error) error {
		real := o.Stdout
		var buf bytes.Buffer
		o.Stdout = &buf
		err := fn()
		o.Stdout = real
		if err != nil {
			return err
		}
		_, _ = fmt.Fprint(o.Stdout, filterLines(buf.String(), o.Arg))
		return nil
	}

	switch o.Cmd {
	case "collections":
		return runFiltered(func() error {
			return mongoEval(`db.getCollectionInfos({type:"collection"}).map(c=>c.name).sort().forEach(n=>print(n))`)
		})
	case "tables":
		return runFiltered(func() error {
			return mysqlRun("SHOW TABLES")
		})
	case "doc":
		if o.Arg == "" {
			_, _ = fmt.Fprintln(o.Stderr, "zenify db-read: doc needs a collection name — run 'zenify db-read collections' first")
			return fmt.Errorf("zenify db-read: arg required")
		}
		return mongoEval("printjson(db.getCollection(" + jsString(o.Arg) + ").findOne())")
	case "count":
		if o.Arg == "" {
			_, _ = fmt.Fprintln(o.Stderr, "zenify db-read: count needs a collection name")
			return fmt.Errorf("zenify db-read: arg required")
		}
		return mongoEval("print(db.getCollection(" + jsString(o.Arg) + ").estimatedDocumentCount())")
	case "eval":
		if o.Arg == "" {
			_, _ = fmt.Fprintln(o.Stderr, "zenify db-read: eval needs a JS expression")
			return fmt.Errorf("zenify db-read: arg required")
		}
		if RefuseWrites(o.Arg) {
			_, _ = fmt.Fprintln(o.Stderr, "zenify db-read: refusing — this looks like a write.")
			_, _ = fmt.Fprintln(o.Stderr, "zenify db-read: writes go in a script that connects and runs, never in a direct query.")
			return fmt.Errorf("zenify db-read: refused write")
		}
		return mongoEval(o.Arg)
	case "sql":
		if o.Arg == "" {
			_, _ = fmt.Fprintln(o.Stderr, "zenify db-read: sql needs a query")
			return fmt.Errorf("zenify db-read: arg required")
		}
		if RefuseWrites(o.Arg) {
			_, _ = fmt.Fprintln(o.Stderr, "zenify db-read: refusing — this looks like a write.")
			_, _ = fmt.Fprintln(o.Stderr, "zenify db-read: writes go in a script that connects and runs, never in a direct query.")
			return fmt.Errorf("zenify db-read: refused write")
		}
		return mysqlRun(o.Arg)
	default:
		_, _ = fmt.Fprintln(o.Stderr, "Usage:")
		_, _ = fmt.Fprintln(o.Stderr, "  db-read collections            list Mongo collections in the selected profile")
		_, _ = fmt.Fprintln(o.Stderr, "  db-read collections <substr>   only names containing <substr>")
		_, _ = fmt.Fprintln(o.Stderr, "  db-read tables                 list MySQL tables in the selected profile")
		_, _ = fmt.Fprintln(o.Stderr, "  db-read tables <substr>        only names containing <substr>")
		_, _ = fmt.Fprintln(o.Stderr, "  db-read doc <collection>       findOne() from that collection")
		_, _ = fmt.Fprintln(o.Stderr, "  db-read count <collection>     estimated document count")
		_, _ = fmt.Fprintln(o.Stderr, "  db-read eval '<js>'            mongosh expression on the selected database")
		_, _ = fmt.Fprintln(o.Stderr, "  db-read sql '<query>'          read-only MySQL query")
		return fmt.Errorf("zenify db-read: unknown subcommand %q", o.Cmd)
	}
}
