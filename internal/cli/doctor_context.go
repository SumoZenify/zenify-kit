package cli

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	claudeMdMaxBytes    = 8192
	claudeMdMaxLines    = 200
	contextBudgetBytes  = 32768
	shadowDetailSuffix  = " — xoá hoặc đổi tên bản cá nhân" //znf:allow-lang
	personalKitDirName  = "znf"
	frontmatterDelim    = "---"
	frontmatterPathsKey = "paths:"
)

// hasPathsFrontmatter reports whether b opens with a `---` frontmatter block
// containing a line that starts with `paths:` (a path-scoped, lazily loaded rule).
func hasPathsFrontmatter(b []byte) bool {
	s := strings.ReplaceAll(string(b), "\r\n", "\n")
	if !strings.HasPrefix(s, frontmatterDelim+"\n") {
		return false
	}
	for _, l := range strings.Split(s[len(frontmatterDelim)+1:], "\n") {
		if l == frontmatterDelim {
			return false
		}
		if strings.HasPrefix(l, frontmatterPathsKey) {
			return true
		}
	}
	return false
}

func dirNames(dir string, wantDir bool) []string {
	ents, _ := os.ReadDir(dir)
	var out []string
	for _, e := range ents {
		isDir := e.IsDir()
		if !isDir && e.Type()&fs.ModeSymlink != 0 {
			if fi, err := os.Stat(filepath.Join(dir, e.Name())); err == nil {
				isDir = fi.IsDir()
			}
		}
		if isDir != wantDir {
			continue
		}
		out = append(out, e.Name())
	}
	sort.Strings(out)
	return out
}

func exists(p string) bool { _, err := os.Stat(p); return err == nil }

// shadowCheck lists personal skills/agents/rules that shadow a kit or workspace one.
func shadowCheck(home, workspace string) Check {
	return Check{
		Name: "shadow",
		Run: func() (bool, string) {
			claude := filepath.Join(home, ".claude")
			kit := filepath.Join(claude, "skills", personalKitDirName)
			var lines []string
			add := func(kind, name string) {
				lines = append(lines, "shadow: "+kind+" "+name+shadowDetailSuffix)
			}
			for _, n := range dirNames(filepath.Join(claude, "skills"), true) {
				if n != personalKitDirName && exists(filepath.Join(kit, "skills", n)) {
					add("skill", n)
				}
			}
			for _, f := range dirNames(filepath.Join(claude, "agents"), false) {
				if strings.HasSuffix(f, ".md") && exists(filepath.Join(kit, "agents", f)) {
					add("agent", strings.TrimSuffix(f, ".md"))
				}
			}
			if workspace != "" {
				for _, f := range dirNames(filepath.Join(claude, "rules"), false) {
					if strings.HasSuffix(f, ".md") && exists(filepath.Join(workspace, ".claude", "rules", f)) {
						add("rule", strings.TrimSuffix(f, ".md"))
					}
				}
			}
			if len(lines) == 0 {
				return true, "shadow: không có" //znf:allow-lang
			}
			return true, "cảnh báo:\n" + strings.Join(lines, "\n") //znf:allow-lang
		},
	}
}

// alwaysLoadedRules sums the bytes of .md rules under dir that have no paths: frontmatter.
func alwaysLoadedRules(dir string) (int, error) {
	total := 0
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if p == dir && os.IsNotExist(err) {
				return filepath.SkipDir
			}
			return err
		}
		if d.IsDir() || !strings.HasSuffix(p, ".md") {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if !hasPathsFrontmatter(b) {
			total += len(b)
		}
		return nil
	})
	return total, err
}

func fileSize(p string) (int, error) {
	b, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return 0, nil
	}
	return len(b), err
}

// contextBudgetCheck reports the bytes of always-loaded instruction files and
// per-repo CLAUDE.md size/line/tree-diagram warnings. repos are paths relative to workspace.
func contextBudgetCheck(home, workspace string, repos []string) Check {
	return Check{
		Name: "context-budget",
		Run: func() (bool, string) {
			if workspace == "" {
				return false, "không tìm thấy workspace" //znf:allow-lang
			}
			total, wsSize := 0, 0
			for _, p := range []string{filepath.Join(workspace, "CLAUDE.md"), filepath.Join(home, ".claude", "CLAUDE.md")} {
				n, err := fileSize(p)
				if err != nil {
					return false, fmt.Sprintf("%s không đọc được: %v", p, err) //znf:allow-lang
				}
				total += n
				if strings.HasPrefix(p, workspace) {
					wsSize = n
				}
			}
			for _, d := range []string{filepath.Join(workspace, ".claude", "rules"), filepath.Join(home, ".claude", "rules")} {
				n, err := alwaysLoadedRules(d)
				if err != nil {
					return false, fmt.Sprintf("%s không đọc được: %v", d, err) //znf:allow-lang
				}
				total += n
			}
			head := fmt.Sprintf("tổng: %d byte", total) //znf:allow-lang
			if total > contextBudgetBytes || wsSize > claudeMdMaxBytes {
				head = "cảnh báo: " + head //znf:allow-lang
				if wsSize > claudeMdMaxBytes {
					head += fmt.Sprintf("; CLAUDE.md workspace %d byte >8 KiB", wsSize)
				}
			}
			lines := []string{head}
			for _, r := range repos {
				p := filepath.Join(workspace, r, "CLAUDE.md")
				label := filepath.ToSlash(r) + "/CLAUDE.md"
				b, err := os.ReadFile(p)
				if os.IsNotExist(err) {
					lines = append(lines, label+": thiếu") //znf:allow-lang
					continue
				}
				if err != nil {
					return false, fmt.Sprintf("%s không đọc được: %v", p, err) //znf:allow-lang
				}
				nl := bytes.Count(b, []byte("\n"))
				if len(b) > 0 && b[len(b)-1] != '\n' {
					nl++
				}
				l := fmt.Sprintf("%s: %d byte, %d dòng", label, len(b), nl) //znf:allow-lang
				if len(b) > claudeMdMaxBytes {
					l += " cảnh báo: >8 KiB" //znf:allow-lang
				}
				if nl > claudeMdMaxLines {
					l += " cảnh báo: >200 dòng" //znf:allow-lang
				}
				if bytes.Contains(b, []byte("├──")) || bytes.Contains(b, []byte("└──")) {
					l += " cảnh báo: cây thư mục" //znf:allow-lang
				}
				lines = append(lines, l)
			}
			return true, strings.Join(lines, "\n")
		},
	}
}

// manifestRepoPaths returns the manifest repos' paths relative to workspace.
func manifestRepoPaths(workspace string) ([]string, error) {
	m, _, err := loadKitManifest("", filepath.Join(workspace, ".zenify-overlay.yaml"))
	if err != nil {
		return nil, err
	}
	var repos []string
	for _, r := range m.Repos {
		p := r.Path
		if p == "" {
			p = filepath.Join("repos", r.Name)
		}
		repos = append(repos, p)
	}
	return repos, nil
}

func registerContextChecks() {
	RegisterCheck(Check{Name: "shadow", Run: func() (bool, string) {
		home, err := os.UserHomeDir()
		if err != nil {
			return false, "cannot resolve HOME"
		}
		cwd, _ := os.Getwd()
		ws, _, _ := resolveWorkspace(cwd, "", os.Getenv, os.UserHomeDir)
		return shadowCheck(home, ws).Run()
	}})
	RegisterCheck(Check{Name: "context-budget", Run: func() (bool, string) {
		home, err := os.UserHomeDir()
		if err != nil {
			return false, "cannot resolve HOME"
		}
		cwd, _ := os.Getwd()
		ws, _, ok := resolveWorkspace(cwd, "", os.Getenv, os.UserHomeDir)
		if !ok {
			return contextBudgetCheck(home, "", nil).Run()
		}
		repos, err := manifestRepoPaths(ws)
		if err != nil {
			return false, fmt.Sprintf("manifest không đọc được: %v", err) //znf:allow-lang
		}
		return contextBudgetCheck(home, ws, repos).Run()
	}})
}
