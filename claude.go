package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Agent is one entry of `claude agents --json --all`.
type Agent struct {
	ID        string `json:"id"`
	SessionID string `json:"sessionId"`
	Cwd       string `json:"cwd"`
	Kind      string `json:"kind"`
	Status    string `json:"status"` // busy | idle | ...
	State     string `json:"state"`  // running | done | ...
	Name      string `json:"name"`
}

func (c Config) claude(dir string, args ...string) *exec.Cmd {
	cmd := exec.Command(c.claudeBin(), args...)
	cmd.Dir = dir
	if d := c.claudeConfigDir(); d != "" {
		cmd.Env = append(os.Environ(), "CLAUDE_CONFIG_DIR="+d)
	}
	return cmd
}

// claudeShellCmd is the shell command line for running claude with args,
// for opening in another terminal window.
func (c Config) claudeShellCmd(dir string, args ...string) string {
	line := "cd " + shellQuote(dir) + " && "
	if d := c.claudeConfigDir(); d != "" {
		line += "CLAUDE_CONFIG_DIR=" + shellQuote(d) + " "
	}
	line += shellQuote(c.claudeBin())
	for _, a := range args {
		line += " " + shellQuote(a)
	}
	return line
}

func (c Config) listAgents() (map[string]Agent, error) {
	out, err := c.claude("", "agents", "--json", "--all").Output()
	if err != nil {
		return nil, fmt.Errorf("claude agents: %w", err)
	}
	var list []Agent
	if err := json.Unmarshal(out, &list); err != nil {
		return nil, err
	}
	byID := map[string]Agent{}
	for _, a := range list {
		if a.ID != "" {
			byID[a.ID] = a
		}
	}
	return byID, nil
}

var bgIDRe = regexp.MustCompile(`backgrounded · ([0-9a-f]+)`)

func parseBgID(out []byte) (string, error) {
	m := bgIDRe.FindSubmatch(out)
	if m == nil {
		return "", fmt.Errorf("unexpected claude output: %s", strings.TrimSpace(string(out)))
	}
	return string(m[1]), nil
}

// stateDir is the git-ignored folder where agents record their progress.
const stateDir = ".swat"

// launch starts a background agent in a fresh worktree of clone.
func (c Config) launch(clone Clone, issue Issue, comments []Comment, pause bool) (*Job, error) {
	if err := excludeFromGit(clone.Path, stateDir+"/"); err != nil {
		return nil, err
	}
	name := fmt.Sprintf("swat-%d", issue.Number)
	for k := 2; dirExists(filepath.Join(clone.Path, ".claude", "worktrees", name)); k++ {
		name = fmt.Sprintf("swat-%d-%d", issue.Number, k)
	}
	args := []string{"--bg", "-w", name, "-n", fmt.Sprintf("%s#%d", issue.Repo, issue.Number)}
	if c.PermissionMode != "" {
		args = append(args, "--permission-mode", c.PermissionMode)
	}
	if c.Model != "" {
		args = append(args, "--model", c.Model)
	}
	args = append(args, buildPrompt(issue, comments, clone, pause))
	out, err := c.claude(clone.Path, args...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("claude --bg: %v: %s", err, strings.TrimSpace(string(out)))
	}
	id, err := parseBgID(out)
	if err != nil {
		return nil, err
	}
	return &Job{
		IssueRepo:      issue.Repo,
		Issue:          issue.Number,
		Repo:           clone.Name,
		RepoPath:       clone.Path,
		WorktreeName:   name,
		WorktreePath:   filepath.Join(clone.Path, ".claude", "worktrees", name),
		Branch:         "worktree-" + name,
		AgentID:        id,
		StateDir:       stateDir,
		PauseAfterPlan: pause,
		StartedAt:      time.Now(),
	}, nil
}

// message continues the job's session in the background with a new prompt.
// An idle background session still counts as running, and resuming one (or
// passing flags) forks a copy, so stop it first and resume without flags:
// that wakes the same session with its saved options. Returns the agent ID.
func (c Config) message(job *Job, sessionID, text string) (string, error) {
	c.claude("", "stop", job.AgentID).Run()
	out, err := c.claude(job.WorktreePath, "--bg", "--resume", sessionID, text).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("claude --resume: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return parseBgID(out)
}

func (c Config) stop(id string) error {
	out, err := c.claude("", "stop", id).CombinedOutput()
	if err != nil {
		return fmt.Errorf("claude stop: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (c Config) remove(id string) error {
	out, err := c.claude("", "rm", id).CombinedOutput()
	if err != nil {
		return fmt.Errorf("claude rm: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// ---- files the agent keeps in <worktree>/<stateDir>/ ----

type AgentStatus struct {
	Stage string `json:"stage"` // research | plan | awaiting_approval | implement | ready | blocked
	Note  string `json:"note"`
}

func readAgentStatus(j *Job) (AgentStatus, bool) {
	var s AgentStatus
	data, err := os.ReadFile(filepath.Join(j.dir(), j.StateDir, "status.json"))
	if err != nil || json.Unmarshal(data, &s) != nil {
		return s, false
	}
	return s, true
}

func readPlan(j *Job) []string {
	data, err := os.ReadFile(filepath.Join(j.dir(), j.StateDir, "PLAN.md"))
	if err != nil {
		return nil
	}
	var steps, all []string
	for _, line := range strings.Split(string(data), "\n") {
		t := strings.TrimSpace(line)
		if t == "" {
			continue
		}
		all = append(all, t)
		if strings.HasPrefix(t, "- [") {
			steps = append(steps, t)
		}
	}
	if len(steps) > 0 {
		return steps
	}
	return all
}

// excludeFromGit adds pattern to the repo's local info/exclude, which every
// worktree shares, keeping it out of git status and commits.
func excludeFromGit(repoPath, pattern string) error {
	out, err := exec.Command("git", "-C", repoPath, "rev-parse", "--path-format=absolute", "--git-common-dir").Output()
	if err != nil {
		return fmt.Errorf("git rev-parse in %s: %w", repoPath, err)
	}
	path := filepath.Join(strings.TrimSpace(string(out)), "info", "exclude")
	data, _ := os.ReadFile(path)
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == pattern {
			return nil
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if len(data) > 0 && !bytes.HasSuffix(data, []byte("\n")) {
		f.WriteString("\n")
	}
	_, err = f.WriteString(pattern + "\n")
	return err
}

// ---- transcript ----

type LogLine struct {
	At   time.Time
	Kind string // tool | text
	Text string
}

func (c Config) transcriptPath(sessionID string) string {
	dir := c.claudeConfigDir()
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".claude")
	}
	matches, _ := filepath.Glob(filepath.Join(dir, "projects", "*", sessionID+".jsonl"))
	if len(matches) == 0 {
		return ""
	}
	return matches[0]
}

// readTranscript returns the last n assistant actions from a session transcript.
func readTranscript(path, wt string, n int) []LogLine {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	const tail = 1 << 20
	if st, err := f.Stat(); err == nil && st.Size() > tail {
		f.Seek(st.Size()-tail, io.SeekStart)
	}
	var lines []LogLine
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), 16<<20)
	for sc.Scan() {
		var e struct {
			Type      string    `json:"type"`
			Timestamp time.Time `json:"timestamp"`
			Message   struct {
				Content json.RawMessage `json:"content"`
			} `json:"message"`
		}
		if json.Unmarshal(sc.Bytes(), &e) != nil || e.Type != "assistant" {
			continue
		}
		var parts []struct {
			Type  string          `json:"type"`
			Text  string          `json:"text"`
			Name  string          `json:"name"`
			Input json.RawMessage `json:"input"`
		}
		if json.Unmarshal(e.Message.Content, &parts) != nil {
			continue
		}
		for _, p := range parts {
			switch p.Type {
			case "tool_use":
				lines = append(lines, LogLine{e.Timestamp, "tool", describeTool(p.Name, p.Input, wt)})
			case "text":
				if t := firstLine(p.Text); t != "" {
					lines = append(lines, LogLine{e.Timestamp, "text", t})
				}
			}
		}
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return lines
}

func describeTool(name string, raw json.RawMessage, wt string) string {
	var in map[string]any
	json.Unmarshal(raw, &in)
	str := func(k string) string {
		s, _ := in[k].(string)
		return s
	}
	rel := func(p string) string {
		if r, err := filepath.Rel(wt, p); err == nil && !strings.HasPrefix(r, "..") {
			return r
		}
		return p
	}
	switch name {
	case "Bash":
		return "Bash " + firstLine(str("command"))
	case "Read", "Edit", "Write", "NotebookEdit":
		return name + " " + rel(str("file_path"))
	case "Grep", "Glob":
		return fmt.Sprintf("%s %q", name, str("pattern"))
	case "Agent", "Task":
		return name + " " + str("description")
	}
	return name
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i] + " …"
	}
	return s
}

// ---- git ----

type GitInfo struct {
	Ahead     int
	ShortStat string
}

func gitInfo(wt string) GitInfo {
	base := "origin/HEAD"
	if out, err := exec.Command("git", "-C", wt, "rev-parse", "--abbrev-ref", "origin/HEAD").Output(); err == nil {
		base = strings.TrimSpace(string(out))
	}
	var gi GitInfo
	if out, err := exec.Command("git", "-C", wt, "rev-list", "--count", base+"..HEAD").Output(); err == nil {
		fmt.Sscan(string(out), &gi.Ahead)
	}
	if out, err := exec.Command("git", "-C", wt, "diff", "--shortstat", base+"...HEAD").Output(); err == nil {
		gi.ShortStat = strings.TrimSpace(string(out))
	}
	return gi
}

func git(dir string, args ...string) error {
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(string(out)))
	}
	return nil
}

func dirExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}
