package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// setting describes one config.json key. It drives `swat config`,
// `swat config init` and `swat config set`.
type setting struct {
	Key     string
	Kind    string // string | bool | list
	Example string // JSON value shown in the starter file
	Help    string
}

var settings = []setting{
	{"query", "string", `"org:acme label:bug"`, "Extra GitHub search qualifiers, added to: is:issue is:open assignee:<you>."},
	{"permissionMode", "string", `"auto"`, "Passed to claude --permission-mode. Agents wait when they need a permission; auto or acceptEdits avoids most stops."},
	{"model", "string", `"opus"`, "Passed to claude --model."},
	{"pauseAfterPlan", "bool", `false`, "Start agents in pause-after-plan mode (you approve the plan before any code changes). Default true."},
	{"editor", "string", `"code"`, "Command that opens a folder (the path is appended). Default: the first of cursor, code, zed, JetBrains IDEs, subl, else open/xdg-open."},
	{"searchPaths", "list", `["~/code", "~/work"]`, "Folders to scan (two levels deep) for git clones with a GitHub origin. Replaces the built-in list."},
	{"repos", "map", `{"acme/api": "~/work/api"}`, "Clones to always offer, as owner/name → path. Manage with `swat repos` instead if you like."},
	{"claudeBin", "string", `"~/.local/bin/claude"`, "The claude executable. Default: claude on your PATH."},
	{"claudeConfigDir", "string", `"~/.claude-work"`, "Run agents with this CLAUDE_CONFIG_DIR, e.g. to use a second Claude account. Default: your normal Claude setup."},
}

func findSetting(key string) (setting, bool) {
	for _, s := range settings {
		if strings.EqualFold(s.Key, key) {
			return s, true
		}
	}
	return setting{}, false
}

// effective returns a setting's value as swat will use it, and where it came from.
func (c Config) effective(key string) (string, string) {
	set := func(v string) (string, string) { return v, "config" }
	switch key {
	case "query":
		if c.Query != "" {
			return set(c.Query)
		}
		return "(none)", "default"
	case "permissionMode":
		if c.PermissionMode != "" {
			return set(c.PermissionMode)
		}
		return "(your Claude default)", "default"
	case "model":
		if c.Model != "" {
			return set(c.Model)
		}
		return "(your Claude default)", "default"
	case "pauseAfterPlan":
		if c.PauseAfterPlan != nil {
			return set(fmt.Sprint(*c.PauseAfterPlan))
		}
		return "true", "default"
	case "editor":
		if c.Editor != "" {
			return set(c.Editor)
		}
		return strings.Join(c.editor(), " "), "detected"
	case "searchPaths":
		if len(c.SearchPaths) > 0 {
			return set(strings.Join(c.SearchPaths, ", "))
		}
		return strings.Join(defaultSearchPaths, ", "), "default"
	case "repos":
		return fmt.Sprintf("%d configured, %d remembered (see `swat repos`)", len(c.Repos), len(c.Remembered)), "config"
	case "claudeBin":
		if c.ClaudeBin != "" {
			return set(c.ClaudeBin)
		}
		return c.claudeBin(), "detected"
	case "claudeConfigDir":
		if c.ClaudeConfigDir != "" {
			return set(c.ClaudeConfigDir)
		}
		return "(your Claude default)", "default"
	}
	return "", ""
}

func configCmd(cfg Config, args []string) error {
	sub := ""
	if len(args) > 0 {
		sub = args[0]
	}
	switch sub {
	case "", "show":
		showConfig(cfg)
		return nil
	case "path":
		fmt.Println(configPath())
		return nil
	case "init":
		force := len(args) > 1 && args[1] == "--force"
		if err := writeStarterConfig(force); err != nil {
			return err
		}
		fmt.Printf("Wrote %s\nEvery setting is commented out; uncomment the ones you want, or run `swat config edit`.\n", configPath())
		return nil
	case "edit":
		if _, err := os.Stat(configPath()); errors.Is(err, os.ErrNotExist) {
			if err := writeStarterConfig(false); err != nil {
				return err
			}
		}
		return openForEditing(cfg, configPath())
	case "set":
		if len(args) != 3 {
			return fmt.Errorf("usage: swat config set <key> <value>")
		}
		return setConfigValue(args[1], &args[2])
	case "unset":
		if len(args) != 2 {
			return fmt.Errorf("usage: swat config unset <key>")
		}
		return setConfigValue(args[1], nil)
	}
	return fmt.Errorf("unknown config command %q (try `swat help config`)", sub)
}

func showConfig(cfg Config) {
	fmt.Printf("Config file: %s", configPath())
	if _, err := os.Stat(configPath()); err != nil {
		fmt.Print("  (not created yet: every setting is at its default)")
	}
	fmt.Print("\n\n")
	width := 0
	for _, s := range settings {
		width = max(width, len(s.Key))
	}
	for _, s := range settings {
		v, src := cfg.effective(s.Key)
		fmt.Printf("  %-*s  %s  %s\n", width, s.Key, v, color("2", "["+src+"]"))
	}
	fmt.Printf("\nIssue search: %s\n", searchQuery("<you>", cfg.Query))
	fmt.Print(`
Change a setting:   swat config set permissionMode auto
Reset to default:   swat config unset permissionMode
Edit the file:      swat config edit
Every setting:      swat help config
`)
}

func starterConfig() string {
	var b strings.Builder
	b.WriteString("// swat settings. Every setting is optional: uncomment the ones you want.\n")
	b.WriteString("// Run `swat config` to see what's in effect, `swat help config` for details.\n{\n")
	for i, s := range settings {
		for _, line := range wrap(s.Help, 74) {
			b.WriteString("  // " + line + "\n")
		}
		comma := ","
		if i == len(settings)-1 {
			comma = ""
		}
		fmt.Fprintf(&b, "  // %q: %s%s\n", s.Key, s.Example, comma)
		if i < len(settings)-1 {
			b.WriteString("\n")
		}
	}
	b.WriteString("}\n")
	return b.String()
}

func writeStarterConfig(force bool) error {
	if _, err := os.Stat(configPath()); err == nil && !force {
		return fmt.Errorf("%s already exists (use `swat config edit`, or `swat config init --force` to replace it)", configPath())
	}
	if err := os.MkdirAll(appDir(), 0o755); err != nil {
		return err
	}
	return os.WriteFile(configPath(), []byte(starterConfig()), 0o644)
}

// setConfigValue sets (or with value nil, removes) one key in config.json.
func setConfigValue(key string, value *string) error {
	s, ok := findSetting(key)
	if !ok {
		var keys []string
		for _, s := range settings {
			keys = append(keys, s.Key)
		}
		return fmt.Errorf("unknown setting %q; settings are: %s", key, strings.Join(keys, ", "))
	}
	if s.Kind == "map" {
		return fmt.Errorf("use `swat repos add <path>` / `swat repos forget <owner/name>`, or `swat config edit`")
	}
	raw := map[string]any{}
	data, err := os.ReadFile(configPath())
	switch {
	case errors.Is(err, os.ErrNotExist):
	case err != nil:
		return err
	default:
		stripped := stripJSONComments(data)
		if !bytes.Equal(bytes.TrimSpace(stripped), bytes.TrimSpace(data)) && hasSettings(stripped) {
			return fmt.Errorf("%s has comments, which `config set` would lose: change it with `swat config edit`", configPath())
		}
		if err := json.Unmarshal(stripped, &raw); err != nil {
			return fmt.Errorf("reading %s: %w", configPath(), err)
		}
	}
	if value == nil {
		delete(raw, s.Key)
	} else {
		switch s.Kind {
		case "bool":
			switch strings.ToLower(*value) {
			case "true", "yes", "on":
				raw[s.Key] = true
			case "false", "no", "off":
				raw[s.Key] = false
			default:
				return fmt.Errorf("%s takes true or false", s.Key)
			}
		case "list":
			var list []string
			for _, p := range strings.Split(*value, ",") {
				if p = strings.TrimSpace(p); p != "" {
					list = append(list, p)
				}
			}
			raw[s.Key] = list
		default:
			raw[s.Key] = *value
		}
	}
	if err := os.MkdirAll(appDir(), 0o755); err != nil {
		return err
	}
	out, _ := json.MarshalIndent(raw, "", "  ")
	if err := os.WriteFile(configPath(), append(out, '\n'), 0o644); err != nil {
		return err
	}
	if value == nil {
		fmt.Printf("Reset %s to its default.\n", s.Key)
	} else {
		fmt.Printf("Set %s.\n", s.Key)
	}
	return nil
}

// hasSettings reports whether a comment-stripped config has any keys, so
// a starter file with everything commented out can still be `set`.
func hasSettings(stripped []byte) bool {
	var m map[string]any
	return json.Unmarshal(stripped, &m) == nil && len(m) > 0
}

func openForEditing(cfg Config, path string) error {
	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor != "" {
		parts := strings.Fields(editor)
		cmd := exec.Command(parts[0], append(parts[1:], path)...)
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
		return cmd.Run()
	}
	parts := cfg.editor()
	fmt.Println("Opening", path)
	return exec.Command(parts[0], append(parts[1:], path)...).Start()
}

// ---- swat repos ----

func reposCmd(cfg Config, args []string) error {
	sub := ""
	if len(args) > 0 {
		sub = args[0]
	}
	switch sub {
	case "", "list":
		clones := discoverClones(cfg)
		names := make([]string, 0, len(clones))
		for n := range clones {
			names = append(names, n)
		}
		sort.Strings(names)
		trusted := claudeTrustedDirs(cfg)
		for _, n := range names {
			c := clones[n]
			flags := ""
			if c.Pinned {
				flags += " pinned"
			}
			if len(c.Project.IssueRepos) > 0 || c.Project.Instructions != "" {
				flags += " .swat.json"
			}
			if t, known := isTrusted(trusted, c.Path); known && !t {
				flags += " " + color("33", "not-trusted")
			}
			fmt.Printf("%-40s %s%s\n", n, c.Path, color("2", flags))
		}
		fmt.Printf("\n%d clones. Pinned ones are offered first; add one with `swat repos add <path>`.\n", len(clones))
		return nil
	case "add":
		if len(args) != 2 {
			return fmt.Errorf("usage: swat repos add <path>")
		}
		path, err := filepath.Abs(expandHome(args[1]))
		if err != nil {
			return err
		}
		name := githubRemote(path)
		if name == "" {
			return fmt.Errorf("%s isn't a git clone with a GitHub origin remote", path)
		}
		if err := cfg.remember(name, path); err != nil {
			return err
		}
		fmt.Printf("Pinned %s → %s\n", name, path)
		return nil
	case "forget", "rm", "remove":
		if len(args) != 2 {
			return fmt.Errorf("usage: swat repos forget <owner/name>")
		}
		name := strings.ToLower(args[1])
		if _, ok := cfg.Remembered[name]; !ok {
			if _, inConfig := cfg.Repos[name]; inConfig {
				return fmt.Errorf("%s is in %s; remove it there (swat config edit)", name, configPath())
			}
			return fmt.Errorf("%s isn't pinned", name)
		}
		if err := cfg.forget(name); err != nil {
			return err
		}
		fmt.Printf("Unpinned %s. It's still offered if it's under a search path.\n", name)
		return nil
	}
	return fmt.Errorf("unknown repos command %q (try `swat help config`)", sub)
}

// ---- swat init ----

const starterProjectConfig = `{
  "issueRepos": [],
  "instructions": ""
}
`

// initProject writes a starter .swat.json at the root of the current repo.
func initProject() error {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return fmt.Errorf("run swat init inside a git repo")
	}
	path := filepath.Join(strings.TrimSpace(string(out)), ".swat.json")
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%s already exists", path)
	}
	if err := os.WriteFile(path, []byte(starterProjectConfig), 0o644); err != nil {
		return err
	}
	fmt.Printf("Wrote %s\n\n", path)
	fmt.Println(`  issueRepos    other repos whose issues get fixed here, e.g. ["acme/issues"]`)
	fmt.Println(`  instructions  added to every agent prompt, e.g. "Run make test before committing."`)
	fmt.Println("\nCommit it so everyone using swat on this repo gets the same setup. See `swat help project`.")
	return nil
}

func wrap(s string, width int) []string {
	var lines []string
	line := ""
	for _, w := range strings.Fields(s) {
		if line != "" && len(line)+1+len(w) > width {
			lines = append(lines, line)
			line = ""
		}
		if line != "" {
			line += " "
		}
		line += w
	}
	if line != "" {
		lines = append(lines, line)
	}
	return lines
}
