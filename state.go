package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Job is what we remember about an agent started for an issue.
type Job struct {
	IssueRepo      string    `json:"issueRepo"` // owner/name the issue lives in
	Issue          int       `json:"issue"`
	Repo           string    `json:"repo"`     // owner/name the fix is made in
	RepoPath       string    `json:"repoPath"` // main clone of Repo
	WorktreeName   string    `json:"worktreeName"`
	WorktreePath   string    `json:"worktreePath"`
	Branch         string    `json:"branch"`
	AgentID        string    `json:"agentId"`
	SessionID      string    `json:"sessionId"`
	StateDir       string    `json:"stateDir"` // the agent's progress folder inside the worktree
	PauseAfterPlan bool      `json:"pauseAfterPlan"`
	StartedAt      time.Time `json:"startedAt"`
	CheckedOut     bool      `json:"checkedOut"` // worktree removed, branch checked out in the main clone
}

func issueKey(repo string, number int) string {
	return fmt.Sprintf("%s#%d", strings.ToLower(repo), number)
}

func (j *Job) key() string { return issueKey(j.IssueRepo, j.Issue) }

func (j *Job) ref() string { return fmt.Sprintf("%s#%d", j.IssueRepo, j.Issue) }

// dir is where the job's progress folder lives: in the worktree, or in a
// saved copy once the branch has been checked out in the main clone.
func (j *Job) dir() string {
	if j.CheckedOut {
		return filepath.Join(appDir(), "artifacts", strings.NewReplacer("/", "_", "#", "_").Replace(j.key()))
	}
	return j.WorktreePath
}

// workDir is where the job's code is checked out right now.
func (j *Job) workDir() string {
	if j.CheckedOut {
		return j.RepoPath
	}
	return j.WorktreePath
}

type State struct {
	Jobs map[string]*Job `json:"jobs"`
}

func statePath() string { return filepath.Join(appDir(), "state.json") }

func loadState() (*State, error) {
	s := &State{Jobs: map[string]*Job{}}
	data, err := os.ReadFile(statePath())
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	if err := json.Unmarshal(data, s); err != nil {
		return s, err
	}
	if s.Jobs == nil {
		s.Jobs = map[string]*Job{}
	}
	return s, nil
}

func (s *State) save() error {
	if err := os.MkdirAll(appDir(), 0o755); err != nil {
		return err
	}
	data, _ := json.MarshalIndent(s, "", "  ")
	tmp := statePath() + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, statePath())
}

// migrateState converts the prototype's jobs, which were keyed by issue
// number within a single issue repo.
func migrateState(oldDir, issuesRepo string, repos map[string]string) {
	data, err := os.ReadFile(filepath.Join(oldDir, "state.json"))
	if err != nil {
		return
	}
	var old struct {
		Jobs map[string]*Job `json:"jobs"`
	}
	if json.Unmarshal(data, &old) != nil {
		return
	}
	s := &State{Jobs: map[string]*Job{}}
	for _, j := range old.Jobs {
		j.IssueRepo = issuesRepo
		j.StateDir = ".bug-agent"
		j.RepoPath = repos[strings.ToLower(j.Repo)]
		if j.CheckedOut {
			os.MkdirAll(filepath.Dir(j.dir()), 0o755)
			os.Rename(filepath.Join(oldDir, "artifacts", fmt.Sprint(j.Issue)), j.dir())
		}
		s.Jobs[j.key()] = j
	}
	s.save()
}
