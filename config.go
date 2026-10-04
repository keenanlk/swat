package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Config is the user's personal settings, ~/.config/swat/config.json.
// Every field is optional; swat works with no config file at all.
type Config struct {
	Query           string            `json:"query,omitempty"`           // extra GitHub search qualifiers, e.g. "org:acme label:bug"
	ClaudeBin       string            `json:"claudeBin,omitempty"`       // default: claude on PATH
	ClaudeConfigDir string            `json:"claudeConfigDir,omitempty"` // CLAUDE_CONFIG_DIR for agents; default: inherit
	Model           string            `json:"model,omitempty"`           // passed to --model
	PermissionMode  string            `json:"permissionMode,omitempty"`  // passed to --permission-mode
	Editor          string            `json:"editor,omitempty"`          // command; the folder is appended
	SearchPaths     []string          `json:"searchPaths,omitempty"`     // where to look for local clones
	Repos           map[string]string `json:"repos,omitempty"`           // owner/name → local clone, always offered
	PauseAfterPlan  *bool             `json:"pauseAfterPlan,omitempty"`  // default for the start form; default true

	// Remembered are clones swat has used, kept in repos.json so swat never
	// rewrites the user's (possibly commented) config.json.
	Remembered map[string]string `json:"-"`
}

// ProjectConfig is an optional .swat.json committed at a repo's root.
type ProjectConfig struct {
	// IssueRepos lists other repos whose issues are usually fixed in this
	// one (e.g. a separate issue tracker repo), so swat suggests it first.
	IssueRepos []string `json:"issueRepos,omitempty"`
	// Instructions are appended to every agent prompt for this repo.
	Instructions string `json:"instructions,omitempty"`
}

func appDir() string {
	if d := os.Getenv("XDG_CONFIG_HOME"); d != "" {
		return filepath.Join(d, "swat")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "swat")
}

func configPath() string { return filepath.Join(appDir(), "config.json") }

func reposPath() string { return filepath.Join(appDir(), "repos.json") }

var defaultSearchPaths = []string{
	"~/code", "~/src", "~/dev", "~/projects", "~/Projects", "~/repos", "~/git", "~/github",
	"~/Developer", "~/workspace", "~/work", "~/go/src/github.com", "~/PhpstormProjects",
	"~/WebstormProjects", "~/IdeaProjects", "~/GolandProjects", "~/PycharmProjects",
}

func loadConfig() (Config, error) {
	var cfg Config
	data, err := os.ReadFile(configPath())
	switch {
	case errors.Is(err, os.ErrNotExist):
		// First run ever: bring settings over from the prototype, if any.
		if !dirExists(appDir()) {
			if migrated, ok := migrateFromBugAgents(); ok {
				cfg = migrated
			}
		}
	case err != nil:
		return cfg, err
	default:
		if err := json.Unmarshal(stripJSONComments(data), &cfg); err != nil {
			return cfg, fmt.Errorf("%w (check the file's JSON, or run `swat config edit`)", err)
		}
	}
	if cfg.Repos == nil {
		cfg.Repos = map[string]string{}
	}
	cfg.Remembered = map[string]string{}
	if data, err := os.ReadFile(reposPath()); err == nil {
		json.Unmarshal(data, &cfg.Remembered)
	}
	return cfg, nil
}

// allRepos is every clone the user configured or swat remembered, with
// the config winning.
func (c Config) allRepos() map[string]string {
	all := map[string]string{}
	for n, p := range c.Remembered {
		all[strings.ToLower(n)] = expandHome(p)
	}
	for n, p := range c.Repos {
		all[strings.ToLower(n)] = expandHome(p)
	}
	return all
}

// remember records a clone in repos.json so it's ranked first next time.
func (c *Config) remember(name, path string) error {
	c.Remembered[name] = path
	return c.saveRemembered()
}

func (c *Config) forget(name string) error {
	delete(c.Remembered, name)
	return c.saveRemembered()
}

func (c *Config) saveRemembered() error {
	if err := os.MkdirAll(appDir(), 0o755); err != nil {
		return err
	}
	data, _ := json.MarshalIndent(c.Remembered, "", "  ")
	return os.WriteFile(reposPath(), append(data, '\n'), 0o644)
}

func (c Config) pauseAfterPlan() bool { return c.PauseAfterPlan == nil || *c.PauseAfterPlan }

// stripJSONComments removes // and /* */ comments outside strings, so
// config files can be documented inline.
func stripJSONComments(in []byte) []byte {
	out := make([]byte, 0, len(in))
	inString, escaped := false, false
	for i := 0; i < len(in); i++ {
		ch := in[i]
		switch {
		case inString:
			out = append(out, ch)
			switch {
			case escaped:
				escaped = false
			case ch == '\\':
				escaped = true
			case ch == '"':
				inString = false
			}
		case ch == '"':
			inString = true
			out = append(out, ch)
		case ch == '/' && i+1 < len(in) && in[i+1] == '/':
			for i < len(in) && in[i] != '\n' {
				i++
			}
			if i < len(in) {
				out = append(out, '\n')
			}
		case ch == '/' && i+1 < len(in) && in[i+1] == '*':
			i += 2
			for i+1 < len(in) && !(in[i] == '*' && in[i+1] == '/') {
				i++
			}
			i++
		default:
			out = append(out, ch)
		}
	}
	return out
}

func (c Config) save() error {
	if err := os.MkdirAll(appDir(), 0o755); err != nil {
		return err
	}
	data, _ := json.MarshalIndent(c, "", "  ")
	return os.WriteFile(configPath(), append(data, '\n'), 0o644)
}

// claudeBin resolves the claude executable.
func (c Config) claudeBin() string {
	if c.ClaudeBin != "" {
		return expandHome(c.ClaudeBin)
	}
	if p, err := exec.LookPath("claude"); err == nil {
		return p
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "bin", "claude")
}

func (c Config) claudeConfigDir() string { return expandHome(c.ClaudeConfigDir) }

// editor resolves the command used to open a folder.
func (c Config) editor() []string {
	if c.Editor != "" {
		return strings.Fields(expandHome(c.Editor))
	}
	for _, e := range []string{"cursor", "code", "zed", "phpstorm", "idea", "webstorm", "goland", "pycharm", "subl"} {
		if p, err := exec.LookPath(e); err == nil {
			return []string{p}
		}
	}
	if runtime.GOOS == "darwin" {
		return []string{"open"}
	}
	return []string{"xdg-open"}
}

func (c Config) searchPaths() []string {
	paths := c.SearchPaths
	if len(paths) == 0 {
		paths = defaultSearchPaths
	}
	out := make([]string, len(paths))
	for i, p := range paths {
		out[i] = expandHome(p)
	}
	return out
}

func readProjectConfig(repoPath string) ProjectConfig {
	var pc ProjectConfig
	if data, err := os.ReadFile(filepath.Join(repoPath, ".swat.json")); err == nil {
		json.Unmarshal(data, &pc)
	}
	return pc
}

func expandHome(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, p[1:])
	}
	return p
}

// migrateFromBugAgents carries settings and running jobs over from the
// prototype, which kept them in ~/.config/bug-agents.
func migrateFromBugAgents() (Config, bool) {
	home, _ := os.UserHomeDir()
	old := filepath.Join(home, ".config", "bug-agents")
	data, err := os.ReadFile(filepath.Join(old, "config.json"))
	if err != nil {
		return Config{}, false
	}
	var oc struct {
		IssuesRepo      string `json:"issuesRepo"`
		Repos           []struct{ Name, Path string }
		ClaudeBin       string `json:"claudeBin"`
		ClaudeConfigDir string `json:"claudeConfigDir"`
		Model           string `json:"model"`
		PermissionMode  string `json:"permissionMode"`
		EditorCmd       string `json:"editorCmd"`
	}
	if json.Unmarshal(data, &oc) != nil {
		return Config{}, false
	}
	cfg := Config{
		ClaudeBin:       oc.ClaudeBin,
		ClaudeConfigDir: oc.ClaudeConfigDir,
		Model:           oc.Model,
		PermissionMode:  oc.PermissionMode,
		Editor:          oc.EditorCmd,
		Repos:           map[string]string{},
	}
	if oc.IssuesRepo != "" {
		cfg.Query = "repo:" + oc.IssuesRepo
	}
	for _, r := range oc.Repos {
		cfg.Repos[strings.ToLower(r.Name)] = r.Path
	}
	if cfg.save() != nil {
		return cfg, true
	}
	migrateState(old, oc.IssuesRepo, cfg.Repos)
	return cfg, true
}
