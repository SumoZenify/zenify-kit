package docsweep

import (
	"os"
	"path"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/ZenifyAIContactCenter/zenify-kit/internal/speclife"
)

// memFS is an in-memory FS: files maps an absolute "/"-path to its content; directories are
// implied by path prefixes. failWrite makes WriteFile fail for the listed paths.
type memFS struct {
	files     map[string][]byte
	failWrite map[string]bool
}

func newMemFS(m map[string]string) *memFS {
	fs := &memFS{files: map[string][]byte{}, failWrite: map[string]bool{}}
	for p, c := range m {
		fs.files[p] = []byte(c)
	}
	return fs
}

func (f *memFS) ReadDir(p string) ([]os.DirEntry, error) {
	prefix := strings.TrimSuffix(p, "/") + "/"
	seen := map[string]bool{}
	var out []os.DirEntry
	for fp := range f.files {
		if !strings.HasPrefix(fp, prefix) {
			continue
		}
		rest := strings.TrimPrefix(fp, prefix)
		name, _, isDir := strings.Cut(rest, "/")
		if seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, dentry{name, isDir})
	}
	if len(out) == 0 {
		return nil, os.ErrNotExist
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name() < out[j].Name() })
	return out, nil
}

func (f *memFS) ReadFile(p string) ([]byte, error) {
	b, ok := f.files[p]
	if !ok {
		return nil, os.ErrNotExist
	}
	return b, nil
}

func (f *memFS) WriteFile(p string, b []byte, _ os.FileMode) error {
	if f.failWrite[p] {
		return os.ErrPermission
	}
	f.files[p] = append([]byte(nil), b...)
	return nil
}

func (f *memFS) MkdirAll(string, os.FileMode) error { return nil }

func (f *memFS) Remove(p string) error {
	if _, ok := f.files[p]; !ok {
		return os.ErrNotExist
	}
	delete(f.files, p)
	return nil
}

func (f *memFS) Rename(oldp, newp string) error {
	b, ok := f.files[oldp]
	if !ok {
		return os.ErrNotExist
	}
	delete(f.files, oldp)
	f.files[newp] = b
	return nil
}

func (f *memFS) Stat(p string) (os.FileInfo, error) {
	if _, ok := f.files[p]; ok {
		return finfo{path.Base(p), false}, nil
	}
	if _, err := f.ReadDir(p); err == nil {
		return finfo{path.Base(p), true}, nil
	}
	return nil, os.ErrNotExist
}

type dentry struct {
	n string
	d bool
}

func (d dentry) Name() string { return d.n }
func (d dentry) IsDir() bool  { return d.d }
func (d dentry) Type() os.FileMode {
	if d.d {
		return os.ModeDir
	}
	return 0
}
func (d dentry) Info() (os.FileInfo, error) { return finfo(d), nil }

type finfo struct {
	n string
	d bool
}

func (i finfo) Name() string { return i.n }
func (i finfo) Size() int64  { return 0 }
func (i finfo) Mode() os.FileMode {
	if i.d {
		return os.ModeDir | 0o755
	}
	return 0o644
}
func (i finfo) ModTime() time.Time { return time.Time{} }
func (i finfo) IsDir() bool        { return i.d }
func (i finfo) Sys() any           { return nil }

func st(p, state string, hasPlan bool) speclife.Status {
	return speclife.Status{Path: p, Repo: speclife.RepoOf(p), State: speclife.State(state), HasPlan: hasPlan}
}

// assertKinds compares Kind+Path (ignoring Detail) of the actions of one kind, in order.
func assertKinds(t *testing.T, acts []Action, kind string, want []Action) {
	t.Helper()
	var got []Action
	for _, a := range acts {
		if a.Kind == kind {
			got = append(got, Action{Kind: a.Kind, Path: a.Path})
		}
	}
	if len(got) != len(want) {
		t.Fatalf("%s: got %v, want %v", kind, got, want)
	}
	for i := range want {
		if got[i] != (Action{Kind: want[i].Kind, Path: want[i].Path}) {
			t.Fatalf("%s: got %v, want %v", kind, got, want)
		}
	}
}

// assertPaths compares the Paths of the actions of one kind, in order.
func assertPaths(t *testing.T, acts []Action, kind string, want []string) {
	t.Helper()
	var got []string
	for _, a := range acts {
		if a.Kind == kind {
			got = append(got, a.Path)
		}
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("%s: got %q, want %q", kind, got, want)
	}
}

func TestPlan_DeletesPlanOnlyForBuiltOrSuperseded(t *testing.T) { // SC-3
	fs := newMemFS(map[string]string{
		"/s/specs/r/2026-01-01-a-design.md": "# a\n", "/s/plans/r/2026-01-01-a.md": "x",
		"/s/specs/r/2026-01-02-b-design.md": "# b\n", "/s/plans/r/2026-01-02-b.md": "x",
		"/s/specs/r/2026-01-03-c-design.md": "# c\n", "/s/plans/r/2026-01-03-c.md": "x",
		"/s/specs/r/2026-01-04-d-design.md": "# d\n", "/s/plans/r/2026-01-04-d.md": "x",
		"/s/specs/r/2026-01-05-e-design.md": "# e\n", "/s/plans/r/2026-01-05-e.md": "x",
	})
	sts := []speclife.Status{st("specs/r/2026-01-01-a-design.md", "built", true), st("specs/r/2026-01-02-b-design.md", "in-progress", true),
		st("specs/r/2026-01-03-c-design.md", "built?", true), st("specs/r/2026-01-04-d-design.md", "unknown", true), st("specs/r/2026-01-05-e-design.md", "superseded", true)}
	acts, _ := Plan(fs, "/s", sts, time.Now())
	want := []Action{{Kind: KindDeletePlan, Path: "plans/r/2026-01-01-a.md"}, {Kind: KindDeletePlan, Path: "plans/r/2026-01-05-e.md"}}
	assertKinds(t, acts, KindDeletePlan, want)
}

func TestPlan_HandoffByDateSegment(t *testing.T) { // SC-4 + Review Focus 2
	now := time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC)
	d10 := now.AddDate(0, 0, -10).Format("2006-01-02")
	fs := newMemFS(map[string]string{"/s/specs/.keep": "", "/s/handoff/2026-01-01-x.md": "", "/s/handoff/" + d10 + "-y.md": "",
		"/s/handoff/ROADMAP.md": "", "/s/handoff/" + d10 + "-qc/04-tab.png": "", "/s/handoff/2026-01-01-qc/05.png": "",
		"/s/handoff/2026-99-99-bad.md": "", "/s/handoff/.DS_Store": ""})
	acts, _ := Plan(fs, "/s", nil, now)
	assertPaths(t, acts, KindDeleteHandoff, []string{"handoff/2026-01-01-qc/05.png", "handoff/2026-01-01-x.md", "handoff/2026-99-99-bad.md", "handoff/ROADMAP.md"})
}

func TestPlan_ReferenceUnlinked(t *testing.T) { // SC-5
	fs := newMemFS(map[string]string{
		"/s/specs/s/2026-01-01-one-design.md": "see reference/r/a.md\n", "/s/plans/s/2026-01-01-one.md": "see reference/r/b.md\n",
		"/s/reference/r/a.md": "see reference/r/c.md\n", "/s/reference/r/b.md": "", "/s/reference/r/c.md": "",
		"/s/.config/rules/00.md": ""})
	acts, _ := Plan(fs, "/s", []speclife.Status{st("specs/s/2026-01-01-one-design.md", "built", true)}, time.Now())
	assertPaths(t, acts, KindDeleteReference, []string{"reference/r/b.md", "reference/r/c.md"})
	assertPaths(t, acts, KindDeletePlan, []string{"plans/s/2026-01-01-one.md"})
}

func TestPlan_MissingDirsAreEmpty(t *testing.T) { // Review Focus 3
	fs := newMemFS(map[string]string{"/s/specs/r/2026-01-01-a-design.md": "# a\n"})
	acts, notes := Plan(fs, "/s", nil, time.Now())
	if len(acts) != 0 || len(notes) != 0 {
		t.Fatalf("acts=%v notes=%v", acts, notes)
	}
}

func TestRewriteHeader(t *testing.T) {
	cases := []struct {
		name, in, want string
		state          speclife.State
		changed        bool
	}{
		{"replace-en", "# T\n\n**Repo:** k\n**Status:** Draft\n\n---\n", "# T\n\n**Repo:** k\n**Status:** built\n\n---\n", "built", true},
		{"insert-after-header", "# T\n\n**Repo:** k\n**Ngày:** 1\n\n---\n", "# T\n\n**Repo:** k\n**Ngày:** 1\n**Trạng thái:** planned\n\n---\n", "planned", true}, //znf:allow-lang
		{"insert-after-h1", "# T\n\nBody\n", "# T\n\n**Trạng thái:** planned\n\nBody\n", "planned", true},                                                         //znf:allow-lang
		{"already-right", "# T\n\n**Trạng thái:** built\n", "# T\n\n**Trạng thái:** built\n", "built", false},                                                     //znf:allow-lang
		{"no-h1", "Body only\n", "Body only\n", "planned", false},
		{"only-first-line", "# T\n\n**Status:** Draft\n\n## B\n**Status:** Draft\n", "# T\n\n**Status:** built\n\n## B\n**Status:** Draft\n", "built", true}, // Review Focus 4
	}
	for _, c := range cases {
		out, changed, _ := rewriteHeader([]byte(c.in), c.state)
		if string(out) != c.want || changed != c.changed {
			t.Errorf("%s: changed=%v\n%q", c.name, changed, out)
		}
	}
}

func TestPlan_HeaderActions(t *testing.T) { // SC-6
	fs := newMemFS(map[string]string{"/s/specs/r/2026-01-01-a-design.md": "# a\n\n**Status:** Draft\n", "/s/specs/r/2026-01-02-b-design.md": "# b\n\n**Repo:** r\n\n---\n",
		"/s/specs/r/2026-01-03-c-design.md": "# c\n\n**Trạng thái:** built\n", "/s/specs/r/2026-01-04-d-design.md": "# d\n"}) //znf:allow-lang
	acts, _ := Plan(fs, "/s", []speclife.Status{st("specs/r/2026-01-01-a-design.md", "built", false), st("specs/r/2026-01-02-b-design.md", "planned", false),
		st("specs/r/2026-01-03-c-design.md", "built", false), st("specs/r/2026-01-04-d-design.md", "unknown", false)}, time.Now())
	assertPaths(t, acts, KindHeader, []string{"specs/r/2026-01-01-a-design.md", "specs/r/2026-01-02-b-design.md"})
}

func TestApplyThenPlanIsEmpty(t *testing.T) { // SC-7
	now := time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC)
	fs := newMemFS(map[string]string{"/s/specs/s/2026-01-01-one-design.md": "# one\n\n**Status:** Draft\n\nsee reference/r/a.md\n",
		"/s/plans/s/2026-01-01-one.md": "see reference/r/b.md\n", "/s/reference/r/a.md": "", "/s/reference/r/b.md": "B", "/s/handoff/2026-01-01-x.md": "X"})
	sts := []speclife.Status{st("specs/s/2026-01-01-one-design.md", "built", true)}
	acts, _ := Plan(fs, "/s", sts, now)
	if notes := Apply(fs, "/s", "/v", acts); len(notes) != 0 {
		t.Fatal(notes)
	}
	for p, want := range map[string]string{"/v/research/r/b.md": "B", "/v/notes/2026-01-01-x.md": "X", "/v/notes/plans/s/2026-01-01-one.md": "see reference/r/b.md\n"} {
		if got, _ := fs.ReadFile(p); string(got) != want {
			t.Errorf("%s = %q", p, got)
		}
	}
	for _, gone := range []string{"/s/plans/s/2026-01-01-one.md", "/s/reference/r/b.md", "/s/handoff/2026-01-01-x.md"} {
		if _, err := fs.Stat(gone); err == nil {
			t.Errorf("%s still there", gone)
		}
	}
	if b, _ := fs.ReadFile("/s/specs/s/2026-01-01-one-design.md"); !strings.Contains(string(b), "**Status:** built") {
		t.Errorf("header not written: %q", b)
	}
	sts[0].HasPlan = false
	if again, _ := Plan(fs, "/s", sts, now); len(again) != 0 {
		t.Fatalf("second plan not empty: %v", again)
	}
}

func TestApplyCopyFailureKeepsFile(t *testing.T) {
	fs := newMemFS(map[string]string{"/s/specs/.keep": "", "/s/handoff/ROADMAP.md": "R"})
	fs.failWrite["/v/notes/ROADMAP.md"] = true
	acts, _ := Plan(fs, "/s", nil, time.Now())
	notes := Apply(fs, "/s", "/v", acts)
	if len(notes) != 1 || !strings.Contains(notes[0], "keep handoff/ROADMAP.md") {
		t.Fatal(notes)
	}
	if _, err := fs.Stat("/s/handoff/ROADMAP.md"); err != nil {
		t.Fatal("deleted despite copy failure")
	}
}
