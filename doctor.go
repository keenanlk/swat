package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// doctor checks everything swat depends on and says how to fix what's missing.
func doctor(cfg Config) int {
	problems := 0
	ok := func(f string, a ...any) { fmt.Printf("  "+color("32", "✓")+" "+f+"\n", a...) }
	bad := func(f string, a ...any) { problems++; fmt.Printf("  "+color("31", "✗")+" "+f+"\n", a...) }
	info := func(f string, a ...any) { fmt.Printf("  "+color("2", "•")+" "+f+"\n", a...) }

	fmt.Printf("swat %s\n\nClaude Code\n", version)
	bin := cfg.claudeBin()
	if out, err := exec.Command(bin, "--version").Output(); err != nil {
		bad("claude not found at %s. Install it: https://docs.claude.com/en/docs/claude-code", bin)
	} else {
		ok("%s (%s)", bin, strings.TrimSpace(string(out)))
		help, _ := exec.Command(bin, "--help").CombinedOutput()
		if strings.Contains(string(help), "--bg") && strings.Contains(string(help), "--worktree") {
			ok("background agents and worktrees supported")
		} else {
			bad("this claude has no --bg/--worktree: run `claude update`")
		}
		if _, err := cfg.listAgents(); err != nil {
			bad("`claude agents --json` failed: %v (are you logged in? run `claude`)", err)
		} else {
			ok("`claude agents` works")
		}
	}
	if d := cfg.claudeConfigDir(); d != "" {
		if dirExists(d) {
			info("using CLAUDE_CONFIG_DIR=%s", d)
		} else {
			bad("claudeConfigDir %s doesn't exist", d)
		}
	}
	if cfg.PermissionMode == "" {
		info("permission mode: your Claude default. Agents stop when they need permission; consider \"permissionMode\": \"auto\" or \"acceptEdits\"")
	} else {
		info("permission mode: %s", cfg.PermissionMode)
	}

	fmt.Println("\nGitHub")
	if gh, err := newGitHub(); err != nil {
		bad("%v", err)
	} else if who, err := gh.login(); err != nil {
		bad("token from %s doesn't work: %v", gh.source, err)
	} else {
		ok("logged in as %s (via %s)", who, gh.source)
		info("issues: %s", searchQuery(who, cfg.Query))
	}

	fmt.Println("\nTools")
	if out, err := exec.Command("git", "--version").Output(); err != nil {
		bad("git not found")
	} else {
		ok("%s", strings.TrimSpace(string(out)))
	}
	info("editor: %s", strings.Join(cfg.editor(), " "))
	if t := terminalName(); t != "" {
		info("agents open in a new %s window", t)
	} else {
		info("this terminal isn't supported for new windows (TERM_PROGRAM=%q); agents attach in place", os.Getenv("TERM_PROGRAM"))
	}

	clones := discoverClones(cfg)
	fmt.Printf("\nLocal clones (%d)\n", len(clones))
	if len(clones) == 0 {
		bad("none found under %s", strings.Join(cfg.searchPaths(), ", "))
		info("add some with \"repos\" or \"searchPaths\" in %s", configPath())
	}
	trusted := claudeTrustedDirs(cfg)
	names := make([]string, 0, len(clones))
	for n := range clones {
		names = append(names, n)
	}
	sort.Strings(names)
	untrusted := 0
	for _, n := range names {
		c := clones[n]
		line := fmt.Sprintf("%s  %s", n, color("2", c.Path))
		if len(c.Project.IssueRepos) > 0 || c.Project.Instructions != "" {
			line += "  .swat.json"
		}
		switch t, known := isTrusted(trusted, c.Path); {
		case !known:
			info("%s", line)
		case t:
			ok("%s", line)
		default:
			untrusted++
			info("%s  %s", line, color("33", "not trusted"))
		}
	}
	if untrusted > 0 {
		info("agents can only start in repos Claude Code trusts: run `claude` in one once and accept the prompt")
	}
	fmt.Printf("\nConfig: %s", configPath())
	if _, err := os.Stat(configPath()); err != nil {
		fmt.Print(" (none, using defaults)")
	}
	fmt.Println()
	if problems > 0 {
		fmt.Printf("\n%d problem(s) found.\n", problems)
		return 1
	}
	fmt.Println("\nAll good.")
	return 0
}

// claudeTrustedDirs reads which folders Claude Code's trust prompt has been
// accepted for. Returns nil if the file can't be read (unknown).
func claudeTrustedDirs(cfg Config) map[string]bool {
	path := filepath.Join(os.Getenv("HOME"), ".claude.json")
	if d := cfg.claudeConfigDir(); d != "" {
		path = filepath.Join(d, ".claude.json")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var f struct {
		Projects map[string]struct {
			HasTrustDialogAccepted bool `json:"hasTrustDialogAccepted"`
		} `json:"projects"`
	}
	if json.Unmarshal(data, &f) != nil {
		return nil
	}
	out := map[string]bool{}
	for p, v := range f.Projects {
		if v.HasTrustDialogAccepted {
			out[p] = true
		}
	}
	return out
}

// isTrusted reports whether dir or one of its parents is trusted, and
// whether that could be determined at all.
func isTrusted(trusted map[string]bool, dir string) (bool, bool) {
	if trusted == nil {
		return false, false
	}
	for d := dir; ; d = filepath.Dir(d) {
		if trusted[d] {
			return true, true
		}
		if d == filepath.Dir(d) {
			return false, true
		}
	}
}
