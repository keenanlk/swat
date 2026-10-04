//go:build screenshots

// Renders the README screenshots from the real UI with demo data.
// Run scripts/screenshots.sh rather than this directly.
package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

const shotW, shotH = 132, 36

func TestScreenshots(t *testing.T) {
	out := os.Getenv("SWAT_SHOTS")
	if out == "" {
		t.Skip("set SWAT_SHOTS to an output folder")
	}
	lipgloss.SetColorProfile(termenv.TrueColor)
	lipgloss.SetHasDarkBackground(true)
	home, _ := os.UserHomeDir()
	code := func(p string) string { return filepath.Join(home, "code", p) }

	planDir := t.TempDir()
	os.MkdirAll(filepath.Join(planDir, stateDir), 0o755)
	os.WriteFile(filepath.Join(planDir, stateDir, "PLAN.md"), []byte(demoPlan), 0o644)

	now := time.Now()
	issues := []Issue{
		demoIssue("acme/web", 1187, "Dashboard filters reset after page refresh", now.Add(-2*time.Hour), "bug", "frontend"),
		demoIssue("acme/api", 412, "Exported reports show raw ISO timestamps", now.Add(-5*time.Hour), "bug", "reports"),
		demoIssue("acme/issues", 88, "Bulk delete times out for workspaces over 10k items", now.Add(-26*time.Hour), "bug", "customer"),
		demoIssue("acme/web", 1201, "Chart tooltips unreadable in dark mode", now.Add(-50*time.Hour), "bug", "a11y"),
		demoIssue("acme/api", 430, "Webhook retries ignore the Retry-After header", now.Add(-70*time.Hour), "bug"),
		demoIssue("acme/mobile", 56, "Notification badge count never clears", now.Add(-90*time.Hour), "bug", "ios"),
	}
	job := func(issueRepo string, n int, repo, id string, started time.Duration) *Job {
		name := "swat-" + itoa(n)
		return &Job{
			IssueRepo: issueRepo, Issue: n, Repo: repo, RepoPath: code(strings.TrimPrefix(repo, "acme/")),
			WorktreeName: name, WorktreePath: filepath.Join(code(strings.TrimPrefix(repo, "acme/")), ".claude", "worktrees", name),
			Branch: "worktree-" + name, AgentID: id, StateDir: stateDir, StartedAt: now.Add(-started),
		}
	}
	st := &State{Jobs: map[string]*Job{}}
	add := func(j *Job) { st.Jobs[j.key()] = j }
	add(job("acme/web", 1187, "acme/web", "7f3a91c2", 14*time.Minute))
	planJob := job("acme/api", 412, "acme/api", "b81e04d7", 9*time.Minute)
	planJob.WorktreePath = planDir
	add(planJob)
	add(job("acme/issues", 88, "acme/api", "c4d2e9a0", 3*time.Minute))
	add(job("acme/web", 1201, "acme/web", "e19b37f5", 41*time.Minute))
	add(job("acme/api", 430, "acme/api", "0a6c58b3", 22*time.Minute))

	agents := map[string]Agent{
		"7f3a91c2": {ID: "7f3a91c2", Status: "busy", State: "running"},
		"b81e04d7": {ID: "b81e04d7", Status: "idle", State: "done"},
		"c4d2e9a0": {ID: "c4d2e9a0", Status: "busy", State: "running"},
		"e19b37f5": {ID: "e19b37f5", Status: "idle", State: "done"},
		"0a6c58b3": {ID: "0a6c58b3", Status: "idle", State: "done"},
	}
	stages := map[string]AgentStatus{
		issueKey("acme/web", 1187):  {Stage: "implement"},
		issueKey("acme/api", 412):   {Stage: "awaiting_approval", Note: "plan ready for review"},
		issueKey("acme/issues", 88): {Stage: "research"},
		issueKey("acme/web", 1201):  {Stage: "ready", Note: "fix committed, see SUMMARY.md"},
		issueKey("acme/api", 430):   {Stage: "blocked", Note: "Should retries cap at 1h when Retry-After is larger?"},
	}
	at := func(hm string) time.Time {
		t, _ := time.ParseInLocation("15:04", hm, time.Local)
		return t
	}
	details := map[string]detail{
		issueKey("acme/web", 1187): {
			plan: []string{
				"- [x] Reproduce with a test: filters lost after reload",
				"- [x] Persist filter state in the URL query string",
				"- [ ] Restore filters from the URL on first render",
				"- [ ] Run the dashboard test suite and lint",
			},
			git: GitInfo{Ahead: 2, ShortStat: "4 files changed, 61 insertions(+), 18 deletions(-)"},
			log: []LogLine{
				{at("10:42"), "text", "Reading the issue and the dashboard filter code."},
				{at("10:43"), "tool", `Grep "useDashboardFilters"`},
				{at("10:43"), "tool", "Read src/dashboard/useDashboardFilters.ts"},
				{at("10:45"), "tool", "Write src/dashboard/__tests__/filters.persist.test.ts"},
				{at("10:46"), "tool", "Bash npm test -- filters.persist"},
				{at("10:46"), "text", "Reproduced: filters live only in component state."},
				{at("10:48"), "tool", "Write .swat/PLAN.md"},
				{at("10:50"), "tool", "Edit src/dashboard/useDashboardFilters.ts"},
				{at("10:52"), "tool", "Bash npm test -- dashboard"},
				{at("10:53"), "tool", `Bash git commit -m "Persist dashboard filters in the URL"`},
				{at("10:55"), "tool", "Edit src/dashboard/DashboardPage.tsx"},
				{at("10:56"), "text", "Restoring filters from the query string on mount…"},
			},
		},
	}
	clones := map[string]Clone{}
	for _, n := range []string{"acme/mobile", "acme/api", "acme/web", "acme/design-system", "acme/infra", "oss/cli-kit"} {
		_, short, _ := strings.Cut(n, "/")
		clones[n] = Clone{Name: n, Path: code(short)}
	}

	base := func() tea.Model {
		cfg := Config{PermissionMode: "auto", Repos: map[string]string{}, Remembered: map[string]string{}}
		var m tea.Model = newModel(cfg, nil, st, "jdoe")
		m, _ = m.Update(tea.WindowSizeMsg{Width: shotW, Height: shotH})
		m, _ = m.Update(issuesMsg{issues: issues})
		m, _ = m.Update(clonesMsg(clones))
		m, _ = m.Update(pollMsg{agents: agents, stages: stages, details: details})
		return m
	}
	key := func(m tea.Model, k string) tea.Model {
		var msg tea.KeyMsg
		switch k {
		case "down":
			msg = tea.KeyMsg{Type: tea.KeyDown}
		default:
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		}
		m, _ = m.Update(msg)
		return m
	}
	write := func(name string, m tea.Model) {
		if err := os.WriteFile(filepath.Join(out, name+".ansi"), []byte(m.View()), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	write("dashboard", base())
	write("plan", key(key(base(), "down"), "v"))
	m := base()
	for i := 0; i < 5; i++ {
		m = key(m, "down")
	}
	write("start", key(m, "s"))
}

func demoIssue(repo string, n int, title string, updated time.Time, labels ...string) Issue {
	i := Issue{Repo: repo, Number: n, Title: title, UpdatedAt: updated,
		HTMLURL: "https://github.com/" + repo + "/issues/" + itoa(n),
		Body:    demoBodies[n]}
	for _, l := range labels {
		i.Labels = append(i.Labels, struct {
			Name string `json:"name"`
		}{l})
	}
	return i
}

var demoBodies = map[int]string{
	1187: "When I set a few filters on the dashboard and refresh the page, every filter goes back to its default.\n\nExpected: the filters stay applied.",
	412:  "Dates in CSV and PDF exports look like 2026-08-29T04:59:59.000Z instead of the formatted dates the app shows.",
	88:   "Customers with large workspaces see a timeout when they select all and delete. Reported by 3 accounts this week.",
	1201: "In dark mode, chart tooltips are dark grey text on a black background.",
	430:  "When a webhook endpoint returns 429 with Retry-After, we retry after 30s anyway and get rate limited again.",
	56: "After reading every notification, the app icon badge still shows the old unread count. " +
		"It only clears after reinstalling, and comes back with the next push.\n\n" +
		"Steps to reproduce:\n1. Receive a few push notifications\n2. Open the app and read them all\n3. Go to the home screen\n\n" +
		"Expected: the badge clears.\n\nSeen on iOS 18 (iPhone 15 and 13). Android looks fine.",
}

func itoa(n int) string { return strconv.Itoa(n) }

const demoPlan = `# Plan: raw ISO timestamps in exported reports

## Root cause

` + "`ReportExporter.formatRow`" + ` writes ` + "`Date`" + ` values with ` + "`toISOString()`" + `, so the
CSV and PDF exports show values like ` + "`2026-08-29T04:59:59.000Z`" + `. Every
other report view formats dates with the workspace's timezone and locale
through ` + "`formatWorkspaceDate`" + `, which the exporter never calls.

## Steps

- [ ] Add a failing test: export a report with a due date and check the CSV cell
- [ ] Format date columns with ` + "`formatWorkspaceDate(value, workspace)`" + ` in ` + "`formatRow`" + `
- [ ] Keep ISO strings in the JSON export, where they're expected
- [ ] Run the reports test suite and lint

## Risks

- Customers may parse the CSV date column. The new format matches what the
  app shows; call it out in the changelog.
`
