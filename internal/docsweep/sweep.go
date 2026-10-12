// Package docsweep plans and applies the knowledge store's lifecycle cleanup: plans of built or
// superseded specs, stale handoffs and unlinked references are deleted (after a local demotion
// copy), and spec status headers are written from speclife state. Pure apart from the FS seam:
// no git, no network.
package docsweep

import (
	"errors"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/ZenifyAIContactCenter/zenify-kit/internal/speclife"
)

// FS is every filesystem operation docsweep needs, behind a seam so tests run in memory.
// Rename backs the temp+rename header write.
type FS interface {
	ReadDir(string) ([]os.DirEntry, error)
	ReadFile(string) ([]byte, error)
	WriteFile(string, []byte, os.FileMode) error
	MkdirAll(string, os.FileMode) error
	Remove(string) error
	Rename(oldpath, newpath string) error
	Stat(string) (os.FileInfo, error)
}

// OSFS implements FS with the os package.
type OSFS struct{}

func (OSFS) ReadDir(p string) ([]os.DirEntry, error)           { return os.ReadDir(p) }
func (OSFS) ReadFile(p string) ([]byte, error)                 { return os.ReadFile(p) }
func (OSFS) WriteFile(p string, b []byte, m os.FileMode) error { return os.WriteFile(p, b, m) }
func (OSFS) MkdirAll(p string, m os.FileMode) error            { return os.MkdirAll(p, m) }
func (OSFS) Remove(p string) error                             { return os.Remove(p) }
func (OSFS) Rename(oldpath, newpath string) error              { return os.Rename(oldpath, newpath) }
func (OSFS) Stat(p string) (os.FileInfo, error)                { return os.Stat(p) }

// Action is one planned change. Path is relative to the store, "/"-separated.
type Action struct {
	Kind, Path, Detail string
}

const (
	KindDeletePlan      = "delete-plan"
	KindDeleteHandoff   = "delete-handoff"
	KindDeleteReference = "delete-reference"
	KindHeader          = "header"
)

// kindOrder sorts actions by the order of the Kind constants above.
var kindOrder = map[string]int{KindDeletePlan: 0, KindDeleteHandoff: 1, KindDeleteReference: 2, KindHeader: 3}

// handoffMaxAge: a dated handoff older than this is deleted.
const handoffMaxAge = 30 * 24 * time.Hour

var datePrefix = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}`)

// Plan computes the sweep's actions from the store contents and the spec statuses. It never
// writes. Notes report per-directory read failures; a missing directory is simply empty.
func Plan(fs FS, store string, statuses []speclife.Status, now time.Time) ([]Action, []string) {
	var acts []Action
	var notes []string
	abs := func(rel string) string { return filepath.Join(store, filepath.FromSlash(rel)) }
	walk := func(dir string) []string {
		files, err := walkFiles(fs, abs(dir))
		if err != nil {
			notes = append(notes, "docs sweep: read "+dir+": "+err.Error())
		}
		out := make([]string, len(files))
		for i, f := range files {
			out[i] = path.Join(dir, f)
		}
		return out
	}

	// Plans of built or superseded specs.
	deletedPlans := map[string]bool{}
	for _, s := range statuses {
		if !s.HasPlan || (s.State != speclife.StateBuilt && s.State != speclife.StateSuperseded) {
			continue
		}
		p := path.Join("plans", speclife.RepoOf(s.Path), planStem(s.Path)+".md")
		if _, err := fs.Stat(abs(p)); err != nil {
			continue
		}
		deletedPlans[p] = true
		acts = append(acts, Action{Kind: KindDeletePlan, Path: p, Detail: "spec " + string(s.State)})
	}

	// Handoffs: undated, or dated older than handoffMaxAge.
	cutoff := now.Add(-handoffMaxAge)
	for _, p := range walk("handoff") {
		date, ok := handoffDate(strings.TrimPrefix(p, "handoff/"))
		switch {
		case !ok:
			acts = append(acts, Action{Kind: KindDeleteHandoff, Path: p, Detail: "undated"})
		case date.Before(cutoff):
			acts = append(acts, Action{Kind: KindDeleteHandoff, Path: p, Detail: "dated " + date.Format("2006-01-02")})
		}
	}

	// References no spec, surviving plan or .config file links by full relative path.
	var sources []string
	for _, dir := range []string{"specs", "plans", ".config"} {
		for _, p := range walk(dir) {
			if deletedPlans[p] {
				continue
			}
			if b, err := fs.ReadFile(abs(p)); err == nil {
				sources = append(sources, string(b))
			} else {
				notes = append(notes, "docs sweep: read "+p+": "+err.Error())
			}
		}
	}
	for _, p := range walk("reference") {
		if !linked(sources, p) {
			acts = append(acts, Action{Kind: KindDeleteReference, Path: p, Detail: "unlinked"})
		}
	}

	sort.SliceStable(acts, func(i, j int) bool {
		if acts[i].Kind != acts[j].Kind {
			return kindOrder[acts[i].Kind] < kindOrder[acts[j].Kind]
		}
		return acts[i].Path < acts[j].Path
	})
	return acts, notes
}

func linked(sources []string, p string) bool {
	for _, src := range sources {
		if strings.Contains(src, p) {
			return true
		}
	}
	return false
}

// handoffDate returns the date of the first segment of rel that starts with a parseable
// YYYY-MM-DD; a date-like prefix that does not parse does not count.
func handoffDate(rel string) (time.Time, bool) {
	for _, seg := range strings.Split(rel, "/") {
		m := datePrefix.FindString(seg)
		if m == "" {
			continue
		}
		if d, err := time.Parse("2006-01-02", m); err == nil {
			return d, true
		}
	}
	return time.Time{}, false
}

// walkFiles lists the regular files under root, recursively, as "/"-paths relative to root.
// Entries whose name starts with "." are skipped (root itself may be a dot dir). A missing root
// is empty, not an error.
func walkFiles(fs FS, root string) ([]string, error) {
	if _, err := fs.Stat(root); errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	var out []string
	var walk func(dir, rel string) error
	walk = func(dir, rel string) error {
		entries, err := fs.ReadDir(dir)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), ".") {
				continue
			}
			r := path.Join(rel, e.Name())
			switch {
			case e.IsDir():
				if err := walk(filepath.Join(dir, e.Name()), r); err != nil {
					return err
				}
			case e.Type().IsRegular():
				out = append(out, r)
			}
		}
		return nil
	}
	err := walk(root, "")
	return out, err
}

// planStem is a spec's mirror plan name: basename without ".md", then without "-design"
// (same derivation as the CLI's planExists).
func planStem(specPath string) string {
	base := strings.TrimSuffix(path.Base(specPath), ".md")
	return strings.TrimSuffix(base, "-design")
}
