package main

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Clone is a local checkout of a GitHub repo.
type Clone struct {
	Name    string // owner/name, lowercase
	Path    string
	Pinned  bool // listed in the user's config (or used before), so ranked first
	Project ProjectConfig
}

// discoverClones finds local clones: configured and remembered ones, plus any git
// repo one or two levels under the search paths with a GitHub origin.
func discoverClones(cfg Config) map[string]Clone {
	found := map[string]Clone{}
	add := func(name, path string, pinned bool) {
		if _, ok := found[name]; !ok {
			found[name] = Clone{Name: name, Path: path, Pinned: pinned, Project: readProjectConfig(path)}
		}
	}
	for name, path := range cfg.allRepos() {
		add(name, path, true)
	}
	for _, root := range cfg.searchPaths() {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
				continue
			}
			dir := filepath.Join(root, e.Name())
			if name := githubRemote(dir); name != "" {
				add(name, dir, false)
				continue
			}
			// owner/repo layouts like ~/github/acme/api
			sub, err := os.ReadDir(dir)
			if err != nil {
				continue
			}
			for _, s := range sub {
				if s.IsDir() && !strings.HasPrefix(s.Name(), ".") {
					if name := githubRemote(filepath.Join(dir, s.Name())); name != "" {
						add(name, filepath.Join(dir, s.Name()), false)
					}
				}
			}
		}
	}
	return found
}

// githubRemote returns owner/name for a main clone (not a worktree) whose
// origin is on github.com, else "".
func githubRemote(dir string) string {
	f, err := os.Open(filepath.Join(dir, ".git", "config"))
	if err != nil {
		return ""
	}
	defer f.Close()
	inOrigin := false
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "[") {
			inOrigin = line == `[remote "origin"]`
			continue
		}
		if k, v, ok := strings.Cut(line, "="); inOrigin && ok && strings.TrimSpace(k) == "url" {
			return parseGitHubURL(strings.TrimSpace(v))
		}
	}
	return ""
}

// parseGitHubURL turns git@github.com:o/r.git, https://github.com/o/r and
// ssh://git@github.com/o/r.git into "o/r".
func parseGitHubURL(u string) string {
	for _, prefix := range []string{"git@github.com:", "https://github.com/", "http://github.com/", "ssh://git@github.com/", "git://github.com/"} {
		if rest, ok := strings.CutPrefix(u, prefix); ok {
			rest = strings.TrimSuffix(strings.TrimSuffix(rest, "/"), ".git")
			if parts := strings.Split(rest, "/"); len(parts) == 2 && parts[0] != "" && parts[1] != "" {
				return strings.ToLower(rest)
			}
		}
	}
	return ""
}

// rankClones orders clones by how likely they are to hold the fix for an
// issue in issueRepo: its own repo, repos that list it in .swat.json, repos
// from the same owner, then the rest; pinned repos first within each group.
func rankClones(clones map[string]Clone, issueRepo string) []Clone {
	issueRepo = strings.ToLower(issueRepo)
	owner, _, _ := strings.Cut(issueRepo, "/")
	score := func(c Clone) int {
		switch {
		case c.Name == issueRepo:
			return 0
		case containsFold(c.Project.IssueRepos, issueRepo):
			return 1
		case strings.HasPrefix(c.Name, owner+"/"):
			return 2
		}
		return 3
	}
	list := make([]Clone, 0, len(clones))
	for _, c := range clones {
		list = append(list, c)
	}
	sort.Slice(list, func(i, j int) bool {
		si, sj := score(list[i]), score(list[j])
		if si != sj {
			return si < sj
		}
		if list[i].Pinned != list[j].Pinned {
			return list[i].Pinned
		}
		return list[i].Name < list[j].Name
	})
	return list
}

func containsFold(list []string, s string) bool {
	for _, x := range list {
		if strings.EqualFold(x, s) {
			return true
		}
	}
	return false
}
